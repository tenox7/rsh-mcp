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

## Claude Code Configuration

Use the Claude Code CLI to add the MCP server:

```bash
claude mcp add rsh-mcp /path/to/rsh-mcp
```

### File editing consideration

rsh-mcp provides read/write facility using `rcp` copy in/out. However this is not very effective for editing files. It's generally advised to make your agent write/edit/modify files locally then copy them over. Even better mount the workspace folder locally and remotely via NFS/SMB so the files can be edited in place.

### Optional: Configuration with Default Settings

You can override defaults if you want to always use the same username/target:

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

### Misc

If you need RSH/RCP server for Windows NT check out [NTRSHD](https://github.com/tenox7/ntrshd)

### Illegal

Written by Claude. Public domain.
