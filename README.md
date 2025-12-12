# rsh-mcp

RSH/RCP MCP server. Execute commands and transfer files on remote hosts via RSH/RCP protocols for vintage systems.

## Install

```bash
go install github.com/tenox7/rsh-mcp@latest
```

## Usage with Claude Code

```bash
claude mcp add rsh-mcp -- go run github.com/tenox7/rsh-mcp@latest
```

Or if installed:

```bash
claude mcp add rsh-mcp $(go env GOPATH)/bin/rsh-mcp
```

## Testing with Inspector

```bash
npx @modelcontextprotocol/inspector -- go run github.com/tenox7/rsh-mcp@latest
```

## Tools

### exec

Execute a command on a remote host via RSH.

- `host` - Remote hostname or IP address
- `command` - Command to execute
- `user` - Remote username (optional, defaults to current user)
- `port` - Port number (optional, defaults to 514)
- `max_lines` - Maximum lines of output (optional, defaults to 1000)
- `max_bytes` - Maximum bytes of output (optional, defaults to 100000)
- `tail` - Return last N lines instead of first N (optional)

### read

Read a file from a remote host via RCP.

- `host` - Remote hostname or IP address
- `path` - Path to remote file
- `user` - Remote username (optional)
- `port` - Port number (optional, defaults to 514)

### write

Write a file to a remote host via RCP.

- `host` - Remote hostname or IP address
- `path` - Path to remote file
- `content` - Content to write
- `user` - Remote username (optional)
- `port` - Port number (optional, defaults to 514)

## Notes

**File editing:** rsh-mcp provides read/write via RCP copy in/out. For effective editing, work locally then copy over, or mount workspace via NFS/SMB.

**Privileged port:** Some RSH/RCP servers require source port <1024. This client does that but may require elevated privileges on some systems.

**Windows NT:** If you need an RSH/RCP server for Windows NT check out [NTRSHD](https://github.com/tenox7/ntrshd)
