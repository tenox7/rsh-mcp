package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tenox7/rsh-mcp/internal/rcmd"
	"github.com/tenox7/rsh-mcp/internal/rcp"
	"github.com/tenox7/rsh-mcp/internal/rsh"
)

type ExecArgs struct {
	Host     string `json:"host" jsonschema:"description:Remote hostname or IP address"`
	User     string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to current user)"`
	Command  string `json:"command" jsonschema:"description:Command to execute on remote host. It may have several lines"`
	Port     string `json:"port,omitempty" jsonschema:"description:Port number (optional, defaults to 514),default:514"`
	Shell    string `json:"shell,omitempty" jsonschema:"description:sh (default) runs the command under /bin/sh whatever the login shell is and returns its exit status; login hands it to the login shell as it is with no exit status (for Windows NT rshd)"`
	Timeout  int    `json:"timeout,omitempty" jsonschema:"description:Seconds after which the call stops waiting and returns the output so far (optional). The command may go on running"`
	Grep     string `json:"grep,omitempty" jsonschema:"description:Return only the lines that match this regular expression (Go syntax). Filtered here so old remote tools don't matter (optional)"`
	MaxLines int    `json:"max_lines,omitempty" jsonschema:"description:Maximum lines of output to return (optional, defaults to 1000),default:1000"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"description:Maximum bytes of output to return (optional, defaults to 100000),default:100000"`
	Tail     bool   `json:"tail,omitempty" jsonschema:"description:Return last N lines instead of first N (optional),default:false"`
}

type ReadFileArgs struct {
	Host string `json:"host" jsonschema:"description:Remote hostname or IP address"`
	User string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to current user)"`
	Path string `json:"path" jsonschema:"description:Path to remote file to read"`
	Port string `json:"port,omitempty" jsonschema:"description:Port number (optional, defaults to 514),default:514"`
}

type WriteFileArgs struct {
	Host    string `json:"host" jsonschema:"description:Remote hostname or IP address"`
	User    string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to current user)"`
	Path    string `json:"path" jsonschema:"description:Path to remote file to write"`
	Content string `json:"content" jsonschema:"description:Content to write to the file"`
	Port    string `json:"port,omitempty" jsonschema:"description:Port number (optional, defaults to 514),default:514"`
}

type DownloadArgs struct {
	Host      string   `json:"host" jsonschema:"description:Remote hostname or IP address"`
	User      string   `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to current user)"`
	Paths     []string `json:"paths" jsonschema:"description:Remote files to copy. * and ? work as in the remote shell"`
	To        string   `json:"to" jsonschema:"description:Local folder to copy them into (an absolute path; made if missing)"`
	Recursive bool     `json:"recursive,omitempty" jsonschema:"description:Copy folders with everything in them (optional)"`
	Port      string   `json:"port,omitempty" jsonschema:"description:Port number (optional, defaults to 514),default:514"`
}

type UploadArgs struct {
	Host      string   `json:"host" jsonschema:"description:Remote hostname or IP address"`
	User      string   `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to current user)"`
	Paths     []string `json:"paths" jsonschema:"description:Local files to copy (absolute paths). Folders need recursive"`
	To        string   `json:"to" jsonschema:"description:Remote folder to copy them into. For one file it may also be the new file name"`
	Recursive bool     `json:"recursive,omitempty" jsonschema:"description:Copy folders with everything in them (optional)"`
	Port      string   `json:"port,omitempty" jsonschema:"description:Port number (optional, defaults to 514),default:514"`
}

type VersionArgs struct{}

func serverVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

func textResult(text string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: isError,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func execText(args ExecArgs, res *rsh.Result) string {
	var b strings.Builder
	b.Write(res.Output)
	if res.Matched < res.Lines {
		fmt.Fprintf(&b, "[%d of %d lines match the grep]\n", res.Matched, res.Lines)
	}
	if res.Cut {
		fmt.Fprintf(&b, "[cut to %d lines, use max_lines, max_bytes, tail or grep]\n", res.Shown)
	}
	if res.TimedOut {
		fmt.Fprintf(&b, "[timed out after %d s, the command may still be running]\n", args.Timeout)
	}
	if res.ExitStatus != rsh.ExitUnknown {
		fmt.Fprintf(&b, "exit status %d\n", res.ExitStatus)
		return b.String()
	}
	if args.Shell != "login" && !res.TimedOut {
		b.WriteString("[no exit status: the remote shell did not finish the command]\n")
	}
	return b.String()
}

func copyText(verb string, rep *rcp.Report) string {
	var b strings.Builder
	var total int64
	for _, f := range rep.Files {
		total += f.Size
	}
	files := "files"
	if len(rep.Files) == 1 {
		files = "file"
	}
	fmt.Fprintf(&b, "%s %d %s, %d bytes\n", verb, len(rep.Files), files, total)
	for _, f := range rep.Files {
		fmt.Fprintf(&b, "%s (%d bytes)\n", f.Path, f.Size)
	}
	for _, e := range rep.Errors {
		fmt.Fprintf(&b, "error: %s\n", e)
	}
	return b.String()
}

func copyResult(tool, verb string, rep *rcp.Report, err error) *mcp.CallToolResult {
	if rep == nil {
		return textResult("RCP "+tool+" failed: "+err.Error(), true)
	}
	text := copyText(verb, rep)
	if err != nil {
		text += "stopped: " + err.Error() + "\n"
	}
	return textResult(text, err != nil || len(rep.Files) == 0)
}

func main() {
	flag.DurationVar(&rcmd.IdleTimeout, "idle-timeout", rcmd.IdleTimeout, "close connection after this much remote silence")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "rsh-mcp",
		Version: serverVersion(),
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "exec",
		Description: "Execute command remotely via RSH protocol. Returns its output (stdout and stderr) and its exit status",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ExecArgs) (*mcp.CallToolResult, any, error) {
		if args.Port == "" {
			args.Port = "514"
		}
		if args.MaxLines <= 0 {
			args.MaxLines = 1000
		}
		if args.MaxBytes <= 0 {
			args.MaxBytes = 100000
		}

		res, err := rsh.Execute(ctx, rsh.Options{
			Host:     args.Host,
			User:     args.User,
			Port:     args.Port,
			Command:  args.Command,
			Shell:    args.Shell,
			Timeout:  time.Duration(args.Timeout) * time.Second,
			Grep:     args.Grep,
			MaxLines: args.MaxLines,
			MaxBytes: args.MaxBytes,
			Tail:     args.Tail,
		})
		if err != nil {
			return textResult("RSH execution failed: "+err.Error(), true), nil, nil
		}
		return textResult(execText(args, res), false), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read",
		Description: "Read file from remote host via RCP protocol",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ReadFileArgs) (*mcp.CallToolResult, any, error) {
		if args.Port == "" {
			args.Port = "514"
		}

		content, err := rcp.ReadFile(args.Host, args.User, args.Path, args.Port)
		if err != nil {
			return textResult("RCP read failed: "+err.Error(), true), nil, nil
		}
		return textResult(string(content), false), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "write",
		Description: "Write file to remote host via RCP protocol",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args WriteFileArgs) (*mcp.CallToolResult, any, error) {
		if args.Port == "" {
			args.Port = "514"
		}

		err := rcp.WriteFile(args.Host, args.User, args.Path, args.Port, []byte(args.Content))
		if err != nil {
			return textResult("RCP write failed: "+err.Error(), true), nil, nil
		}
		return textResult("File written successfully", false), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "download",
		Description: "Copy files from a remote host to a local folder via RCP protocol: many at once, binary files too",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DownloadArgs) (*mcp.CallToolResult, any, error) {
		if args.Port == "" {
			args.Port = "514"
		}
		if len(args.Paths) == 0 || args.To == "" {
			return textResult("download needs paths and to", true), nil, nil
		}

		rep, err := rcp.Download(ctx, args.Host, args.User, args.Port, args.Paths, args.To, args.Recursive)
		return copyResult("download", "downloaded", rep, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "upload",
		Description: "Copy local files to a remote host via RCP protocol: many at once, binary files too",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UploadArgs) (*mcp.CallToolResult, any, error) {
		if args.Port == "" {
			args.Port = "514"
		}
		if len(args.Paths) == 0 || args.To == "" {
			return textResult("upload needs paths and to", true), nil, nil
		}

		rep, err := rcp.Upload(ctx, args.Host, args.User, args.Port, args.Paths, args.To, args.Recursive)
		return copyResult("upload", "uploaded", rep, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "version",
		Description: "Report the rsh-mcp server version and Go build info",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args VersionArgs) (*mcp.CallToolResult, any, error) {
		text := "rsh-mcp " + serverVersion()
		if info, ok := debug.ReadBuildInfo(); ok {
			text += "\nmodule " + info.Path + "\n" + info.GoVersion
		}
		return textResult(text, false), nil, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
