package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tenox7/rsh-mcp/internal/rcp"
	"github.com/tenox7/rsh-mcp/internal/rsh"
)

type ExecArgs struct {
	Host     string `json:"host" jsonschema:"description:Remote hostname or IP address"`
	User     string `json:"user,omitempty" jsonschema:"description:Remote username (optional, defaults to current user)"`
	Command  string `json:"command" jsonschema:"description:Command to execute on remote host"`
	Port     string `json:"port,omitempty" jsonschema:"description:Port number (optional, defaults to 514),default:514"`
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

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "rsh-mcp",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "exec",
		Description: "Execute command remotely via RSH protocol",
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

		output, err := rsh.Execute(args.Host, args.User, args.Command, args.Port, args.MaxLines, args.MaxBytes, args.Tail)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "RSH execution failed: " + err.Error()},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(output)},
			},
		}, nil, nil
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
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "RCP read failed: " + err.Error()},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(content)},
			},
		}, nil, nil
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
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "RCP write failed: " + err.Error()},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "File written successfully"},
			},
		}, nil, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}