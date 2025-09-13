# RSH-MCP Server

An MCP (Model Context Protocol) server that provides remote shell and file operations using the legacy RSH/RCP protocols for connecting to vintage computer systems.

## Features

- **exec**: Execute commands remotely via RSH protocol
- **read**: Read files from remote hosts via RCP protocol
- **write**: Write files to remote hosts via RCP protocol

## Installation

### Using go install

```bash
go install github.com/tenox7/rsh-mcp@latest
```

This will install the `rsh-mcp` binary to your `$GOPATH/bin` directory (typically `~/go/bin`).

### From Source

Clone the repository and build:

```bash
git clone https://github.com/tenox7/rsh-mcp.git
cd rsh-mcp
go build -o rsh-mcp
```

## Building

```bash
go build -o rsh-mcp
```

## Tools

### exec
Execute commands on remote hosts via RSH.

Parameters:
- `host` (required): Remote hostname or IP address
- `command` (required): Command to execute
- `user` (optional): Remote username (defaults to current user)
- `port` (optional): Port number (defaults to 514)

### read
Read files from remote hosts via RCP.

Parameters:
- `host` (required): Remote hostname or IP address
- `path` (required): Path to remote file
- `user` (optional): Remote username (defaults to current user)
- `port` (optional): Port number (defaults to 514)

### write
Write files to remote hosts via RCP.

Parameters:
- `host` (required): Remote hostname or IP address
- `path` (required): Path to remote file
- `content` (required): Content to write
- `user` (optional): Remote username (defaults to current user)
- `port` (optional): Port number (defaults to 514)

## Claude Code Configuration

Add this to your Claude Code configuration:

### Claude Desktop

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "rsh-mcp": {
      "command": "rsh-mcp",
      "args": []
    }
  }
}
```

Note: If you installed via `go install`, make sure `~/go/bin` is in your PATH, or use the full path like `/path/to/rsh-mcp`.

### Claude Code

Use the Claude Code CLI to add the MCP server:

```bash
claude mcp add rsh-mcp /path/to/rsh-mcp
```

### Usage

**Recommended approach:** Specify connection parameters directly in each tool call for maximum flexibility and clarity:

```
exec(host="192.168.1.100", user="admin", command="ls -la")
read(host="192.168.1.100", user="admin", path="/etc/hosts")
write(host="192.168.1.100", user="admin", path="/tmp/test.txt", content="hello")
```

### Optional: Configuration with Default Settings

**Note: Setting defaults is generally not recommended as it reduces flexibility and clarity.**

If you must set defaults, you can use environment variables:

```json
{
  "mcpServers": {
    "rsh-mcp": {
      "command": "rsh-mcp",
      "args": [],
      "env": {
        "RSH_DEFAULT_HOST": "192.168.1.100",
        "RSH_DEFAULT_USER": "your_username",
        "RSH_DEFAULT_PORT": "514"
      }
    }
  }
}
```

**Important:** Always specify the host and user explicitly in tool calls rather than relying on defaults. This ensures clarity about which machine you're connecting to and prevents accidental connections to wrong hosts.

### Misc

If you need RSH/RCP server for Windows NT check out [NTRSHD](https://github.com/tenox7/ntrshd)
