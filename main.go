package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tenox7/rsh-mcp/internal/rcmd"
	"github.com/tenox7/rsh-mcp/internal/rcp"
	"github.com/tenox7/rsh-mcp/internal/rsh"
	"github.com/tenox7/rsh-mcp/internal/xferlog"
)

type ExecArgs struct {
	Host      string `json:"host,omitempty" jsonschema:"description:Remote hostname or IP address (optional if RSH_HOST is set)"`
	User      string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to RSH_USER, else the local username)"`
	LocalUser string `json:"local_user,omitempty" jsonschema:"description:Local username sent to the remote, which must trust it (optional, defaults to RSH_LOCAL_USER, else the current user)"`
	Command   string `json:"command" jsonschema:"description:Command to execute on remote host. It may have several lines"`
	Port     any    `json:"port,omitempty" jsonschema:"description:Port number as a string or a number (optional, defaults to 514),default:514"`
	Shell    string `json:"shell,omitempty" jsonschema:"description:sh (default) runs the command under /bin/sh whatever the login shell is and returns its exit status; on Windows NT that needs ntrshd 1.2 or later. login hands it to the login shell as it is with no exit status (for other systems without /bin/sh)"`
	Timeout  int    `json:"timeout,omitempty" jsonschema:"description:Seconds after which the call stops waiting and returns the output so far (optional). The command may go on running"`
	Grep     string `json:"grep,omitempty" jsonschema:"description:Return only the lines that match this regular expression (Go syntax). Filtered here so old remote tools don't matter (optional)"`
	MaxLines int    `json:"max_lines,omitempty" jsonschema:"description:Maximum lines of output to return (optional, defaults to 1000),default:1000"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"description:Maximum bytes of output to return (optional, defaults to 100000),default:100000"`
	Tail     bool   `json:"tail,omitempty" jsonschema:"description:Return last N lines instead of first N (optional),default:false"`
}

type ReadFileArgs struct {
	Host      string `json:"host,omitempty" jsonschema:"description:Remote hostname or IP address (optional if RSH_HOST is set)"`
	User      string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to RSH_USER, else the local username)"`
	LocalUser string `json:"local_user,omitempty" jsonschema:"description:Local username sent to the remote, which must trust it (optional, defaults to RSH_LOCAL_USER, else the current user)"`
	Path      string `json:"path" jsonschema:"description:Path to remote file to read"`
	Port     any    `json:"port,omitempty" jsonschema:"description:Port number as a string or a number (optional, defaults to 514),default:514"`
	Encoding string `json:"encoding,omitempty" jsonschema:"description:text (default) returns the file as text and fails if it is not UTF-8. base64 returns any file, binary too, base64 encoded"`
}

type WriteFileArgs struct {
	Host          string `json:"host,omitempty" jsonschema:"description:Remote hostname or IP address (optional if RSH_HOST is set)"`
	User          string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to RSH_USER, else the local username)"`
	LocalUser     string `json:"local_user,omitempty" jsonschema:"description:Local username sent to the remote, which must trust it (optional, defaults to RSH_LOCAL_USER, else the current user)"`
	Path          string `json:"path" jsonschema:"description:Path to remote file to write"`
	Content       string `json:"content,omitempty" jsonschema:"description:Text to write to the file"`
	ContentBase64 string `json:"content_base64,omitempty" jsonschema:"description:Binary content to write, base64 encoded, instead of content"`
	Mode          string `json:"mode,omitempty" jsonschema:"description:Octal file mode on the remote, e.g. 0755 (optional, defaults to 0644)"`
	Port          any    `json:"port,omitempty" jsonschema:"description:Port number as a string or a number (optional, defaults to 514),default:514"`
}

type DownloadArgs struct {
	Host      string   `json:"host,omitempty" jsonschema:"description:Remote hostname or IP address (optional if RSH_HOST is set)"`
	User      string   `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to RSH_USER, else the local username)"`
	LocalUser string   `json:"local_user,omitempty" jsonschema:"description:Local username sent to the remote, which must trust it (optional, defaults to RSH_LOCAL_USER, else the current user)"`
	Paths     []string `json:"paths" jsonschema:"description:Remote files to copy. * and ? work as in the remote shell"`
	To        string   `json:"to" jsonschema:"description:Local folder to copy them into (an absolute path; made if missing)"`
	Recursive bool     `json:"recursive,omitempty" jsonschema:"description:Copy folders with everything in them (optional)"`
	Shell     string   `json:"shell,omitempty" jsonschema:"description:sh (default) quotes unusual remote paths for /bin/sh. login sends them as given, one rcp session per path, for remotes without /bin/sh such as OpenVMS"`
	Port      any      `json:"port,omitempty" jsonschema:"description:Port number as a string or a number (optional, defaults to 514),default:514"`
}

type UploadArgs struct {
	Host      string   `json:"host,omitempty" jsonschema:"description:Remote hostname or IP address (optional if RSH_HOST is set)"`
	User      string   `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to RSH_USER, else the local username)"`
	LocalUser string   `json:"local_user,omitempty" jsonschema:"description:Local username sent to the remote, which must trust it (optional, defaults to RSH_LOCAL_USER, else the current user)"`
	Paths     []string `json:"paths" jsonschema:"description:Local files to copy (absolute paths). Folders need recursive"`
	To        string   `json:"to" jsonschema:"description:Remote folder to copy them into. For one file it may also be the new file name"`
	Recursive bool     `json:"recursive,omitempty" jsonschema:"description:Copy folders with everything in them (optional)"`
	Shell     string   `json:"shell,omitempty" jsonschema:"description:sh (default) quotes unusual remote paths for /bin/sh. login sends them as given, one rcp session per path, for remotes without /bin/sh such as OpenVMS"`
	Port      any      `json:"port,omitempty" jsonschema:"description:Port number as a string or a number (optional, defaults to 514),default:514"`
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
	nothingDone := rep == nil || len(rep.Files)+len(rep.Errors) == 0
	if err != nil && nothingDone {
		return textResult("RCP "+tool+" failed: "+err.Error(), true)
	}
	text := copyText(verb, rep)
	if err != nil {
		text += "stopped: " + err.Error() + "\n"
	}
	return textResult(text, err != nil || len(rep.Files) == 0)
}

// transfers logs each operation when -log-dir or RSH_LOG_DIR is set.
var transfers *xferlog.Logger

func logUsers(user, localUser string) (string, string) {
	def, _ := rcmd.LocalUser()
	if localUser == "" {
		localUser = def
	}
	if user == "" {
		user = localUser
	}
	return user, localUser
}

func copied(rep *rcp.Report) (int64, []xferlog.File) {
	if rep == nil {
		return 0, nil
	}
	var total int64
	var files []xferlog.File
	for _, f := range rep.Files {
		total += f.Size
		files = append(files, xferlog.File{Path: f.Path, Size: f.Size, SHA256: xferlog.SumFile(f.Path)})
	}
	return total, files
}

func copyErr(rep *rcp.Report, err error) string {
	var errs []string
	if rep != nil {
		errs = rep.Errors
		if len(rep.Files) == 0 && err == nil {
			errs = append(errs, "nothing copied")
		}
	}
	if err != nil {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, "; ")
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// portString accepts the port as a JSON string or number, since some
// MCP clients send numbers.
func portString(v any) string {
	switch p := v.(type) {
	case nil:
		return "514"
	case string:
		if p == "" {
			return "514"
		}
		return p
	case float64:
		return strconv.FormatFloat(p, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func envDefault(v *string, name string) {
	if *v == "" {
		*v = os.Getenv(name)
	}
}

// resolveDefaults fills host, user and local_user from the environment and
// returns the port to use.
func resolveDefaults(host, user, localUser *string, port any) (string, error) {
	envDefault(host, "RSH_HOST")
	envDefault(user, "RSH_USER")
	envDefault(localUser, "RSH_LOCAL_USER")
	p := portString(port)
	if e := os.Getenv("RSH_PORT"); e != "" && portUnset(port) {
		p = e
	}
	if *host == "" {
		return "", errors.New("host is required (or set RSH_HOST)")
	}
	return p, nil
}

func portUnset(port any) bool {
	switch v := port.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case float64:
		return v == 514
	default:
		return portString(port) == "514"
	}
}

func main() {
	flag.DurationVar(&rcmd.IdleTimeout, "idle-timeout", rcmd.IdleTimeout, "close connection after this much remote silence")
	flag.BoolVar(&rcmd.RequirePrivilegedPort, "require-privileged-port", rcmd.RequirePrivilegedPort, "fail when no reserved source port can be bound, instead of using an unprivileged one (or RSH_REQUIRE_PRIVPORT=1)")
	logDir := flag.String("log-dir", os.Getenv("RSH_LOG_DIR"), "append a line per operation to rsh_transfers.log in this folder (or RSH_LOG_DIR; off when empty)")
	flag.Parse()

	var err error
	if transfers, err = xferlog.Open(*logDir); err != nil {
		log.Fatalf("log: %v", err)
	}

	if err := newServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func newServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "rsh-mcp",
		Version: serverVersion(),
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "exec",
		Description: "Execute command remotely via RSH protocol. Returns its output (stdout and stderr) and its exit status",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ExecArgs) (*mcp.CallToolResult, any, error) {
		port, err := resolveDefaults(&args.Host, &args.User, &args.LocalUser, args.Port)
		if err != nil {
			return textResult(err.Error(), true), nil, nil
		}
		envDefault(&args.Shell, "RSH_SHELL")
		if args.MaxLines <= 0 {
			args.MaxLines = 1000
		}
		if args.MaxBytes <= 0 {
			args.MaxBytes = 100000
		}

		start := time.Now()
		res, err := rsh.Execute(ctx, rsh.Options{
			Host:      args.Host,
			User:      args.User,
			LocalUser: args.LocalUser,
			Port:      port,
			Command:   args.Command,
			Shell:     args.Shell,
			Timeout:  time.Duration(args.Timeout) * time.Second,
			Grep:     args.Grep,
			MaxLines: args.MaxLines,
			MaxBytes: args.MaxBytes,
			Tail:     args.Tail,
		})
		user, local := logUsers(args.User, args.LocalUser)
		e := xferlog.Entry{Op: "exec", Host: args.Host, Port: port, User: user, LocalUser: local,
			Command: args.Command, Duration: time.Since(start), Err: errText(err)}
		if err != nil {
			transfers.Log(e)
			return textResult("RSH execution failed: "+err.Error(), true), nil, nil
		}
		e.Bytes, e.SHA256, e.Exit = int64(len(res.Output)), xferlog.Sum(res.Output), "unknown"
		if res.ExitStatus != rsh.ExitUnknown {
			e.Exit = strconv.Itoa(res.ExitStatus)
		}
		transfers.Log(e)
		return textResult(execText(args, res), false), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read",
		Description: "Read file from remote host via RCP protocol",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ReadFileArgs) (*mcp.CallToolResult, any, error) {
		port, err := resolveDefaults(&args.Host, &args.User, &args.LocalUser, args.Port)
		if err != nil {
			return textResult(err.Error(), true), nil, nil
		}

		if args.Encoding != "" && args.Encoding != "text" && args.Encoding != "base64" {
			return textResult("encoding must be text or base64", true), nil, nil
		}

		start := time.Now()
		content, err := rcp.ReadFile(args.Host, args.User, args.LocalUser, args.Path, port)
		user, local := logUsers(args.User, args.LocalUser)
		transfers.Log(xferlog.Entry{Op: "read", Host: args.Host, Port: port, User: user, LocalUser: local,
			Paths: []string{args.Path}, Bytes: int64(len(content)), SHA256: xferlog.Sum(content),
			Duration: time.Since(start), Err: errText(err)})
		if err != nil {
			return textResult("RCP read failed: "+err.Error(), true), nil, nil
		}
		if args.Encoding == "base64" {
			return textResult(base64.StdEncoding.EncodeToString(content), false), nil, nil
		}
		if !utf8.Valid(content) {
			return textResult(fmt.Sprintf("%s is not UTF-8 text (%d bytes): read it with encoding base64", args.Path, len(content)), true), nil, nil
		}
		return textResult(string(content), false), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "write",
		Description: "Write file to remote host via RCP protocol",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args WriteFileArgs) (*mcp.CallToolResult, any, error) {
		port, err := resolveDefaults(&args.Host, &args.User, &args.LocalUser, args.Port)
		if err != nil {
			return textResult(err.Error(), true), nil, nil
		}

		content := []byte(args.Content)
		if args.ContentBase64 != "" {
			if args.Content != "" {
				return textResult("give content or content_base64, not both", true), nil, nil
			}
			var err error
			if content, err = base64.StdEncoding.DecodeString(args.ContentBase64); err != nil {
				return textResult("content_base64: "+err.Error(), true), nil, nil
			}
		}
		mode := uint64(0644)
		if args.Mode != "" {
			var err error
			if mode, err = strconv.ParseUint(args.Mode, 8, 32); err != nil || mode > 0777 {
				return textResult("mode must be octal, e.g. 0644", true), nil, nil
			}
		}

		start := time.Now()
		writeErr := rcp.WriteFile(args.Host, args.User, args.LocalUser, args.Path, port, content, os.FileMode(mode))
		user, local := logUsers(args.User, args.LocalUser)
		transfers.Log(xferlog.Entry{Op: "write", Host: args.Host, Port: port, User: user, LocalUser: local,
			Paths: []string{args.Path}, Bytes: int64(len(content)), SHA256: xferlog.Sum(content),
			Duration: time.Since(start), Err: errText(writeErr)})
		if writeErr != nil {
			return textResult("RCP write failed: "+writeErr.Error(), true), nil, nil
		}
		return textResult(fmt.Sprintf("File written successfully, %d bytes", len(content)), false), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "download",
		Description: "Copy files from a remote host to a local folder via RCP protocol: many at once, binary files too",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DownloadArgs) (*mcp.CallToolResult, any, error) {
		port, err := resolveDefaults(&args.Host, &args.User, &args.LocalUser, args.Port)
		if err != nil {
			return textResult(err.Error(), true), nil, nil
		}
		envDefault(&args.Shell, "RSH_SHELL")
		if len(args.Paths) == 0 || args.To == "" {
			return textResult("download needs paths and to", true), nil, nil
		}

		start := time.Now()
		rep, err := rcp.Download(ctx, args.Host, args.User, args.LocalUser, port, args.Paths, args.To, args.Recursive, args.Shell == "login")
		user, local := logUsers(args.User, args.LocalUser)
		le := xferlog.Entry{Op: "download", Host: args.Host, Port: port, User: user, LocalUser: local,
			Paths: args.Paths, To: args.To, Duration: time.Since(start), Err: copyErr(rep, err)}
		le.Bytes, le.Files = copied(rep)
		transfers.Log(le)
		return copyResult("download", "downloaded", rep, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "upload",
		Description: "Copy local files to a remote host via RCP protocol: many at once, binary files too",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args UploadArgs) (*mcp.CallToolResult, any, error) {
		port, err := resolveDefaults(&args.Host, &args.User, &args.LocalUser, args.Port)
		if err != nil {
			return textResult(err.Error(), true), nil, nil
		}
		envDefault(&args.Shell, "RSH_SHELL")
		if len(args.Paths) == 0 || args.To == "" {
			return textResult("upload needs paths and to", true), nil, nil
		}

		start := time.Now()
		rep, err := rcp.Upload(ctx, args.Host, args.User, args.LocalUser, port, args.Paths, args.To, args.Recursive, args.Shell == "login")
		user, local := logUsers(args.User, args.LocalUser)
		le := xferlog.Entry{Op: "upload", Host: args.Host, Port: port, User: user, LocalUser: local,
			Paths: args.Paths, To: args.To, Duration: time.Since(start), Err: copyErr(rep, err)}
		le.Bytes, le.Files = copied(rep)
		transfers.Log(le)
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

	return server
}
