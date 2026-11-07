# rsh-mcp

RSH/RCP MCP server. Execute commands and transfer files on remote hosts via RSH/RCP protocols for vintage systems.

## Usage with Claude Code

```bash
claude mcp add -s user rsh-mcp -- go run github.com/tenox7/rsh-mcp@latest
```

## Testing with Inspector

```bash
npx @modelcontextprotocol/inspector -- go run github.com/tenox7/rsh-mcp@latest
```

## Tools

### exec

Execute a command on a remote host via RSH. Returns its output (stdout and stderr) and its exit status.

- `host` - Remote hostname or IP address
- `command` - Command to execute; it may have several lines
- `user` - Remote username (optional, defaults to current user)
- `port` - Port number, as a string or a number (optional, defaults to 514)
- `shell` - `sh` (default) or `login` (optional, see below)
- `timeout` - Seconds before the call returns the output so far (optional); the command may go on running
- `grep` - Return only the lines that match this regular expression, Go syntax (optional)
- `max_lines` - Maximum lines of output (optional, defaults to 1000)
- `max_bytes` - Maximum bytes of output (optional, defaults to 100000)
- `tail` - Return last N lines instead of first N (optional)

With `shell` `sh`, the command is sent as a script to `/bin/sh` on its standard input, so the login shell (sh, ksh, csh) never parses it. Quotes, `!`, comments and here-documents work the same everywhere, the command reads `/dev/null` as its input, and the exit status comes back. Windows NT has no `/bin/sh`, but [ntrshd](https://github.com/tenox7/ntrshd) 1.2 and later read that script themselves: they run the command with `cmd /c` (several lines become one command line, joined with `&`) and send the exit status back the same way, so `exec` needs no options there either. An older NT rshd answers that it doesn't know `sh`; then nothing runs and `exec` says so. `login` passes the command to the login shell as it is, with no exit status: use it for other hosts without `/bin/sh`, or an older NT rshd.

Output comes back with plain newlines: CRLF line ends become LF, so `grep` works the same on NT output.

`grep` filters here, not on the remote host, so it works with old tools and saves output.

### download

Copy files from a remote host to a local folder via RCP: many at once, binary files too.

- `host` - Remote hostname or IP address
- `paths` - Remote files; `*` and `?` work as in the remote shell
- `to` - Local folder (absolute path), made if missing
- `recursive` - Copy folders with everything in them (optional)
- `user` - Remote username (optional)
- `port` - Port number, as a string or a number (optional, defaults to 514)

### upload

Copy local files to a remote host via RCP: many at once, binary files too. File modes are kept.

- `host` - Remote hostname or IP address
- `paths` - Local files (absolute paths); folders need `recursive`
- `to` - Remote folder; for one file it may also be the new file name
- `recursive` - Copy folders with everything in them (optional)
- `user` - Remote username (optional)
- `port` - Port number, as a string or a number (optional, defaults to 514)

### read

Read a file from a remote host via RCP.

- `host` - Remote hostname or IP address
- `path` - Path to remote file
- `user` - Remote username (optional)
- `port` - Port number, as a string or a number (optional, defaults to 514)

### write

Write a file to a remote host via RCP.

- `host` - Remote hostname or IP address
- `path` - Path to remote file
- `content` - Content to write
- `user` - Remote username (optional)
- `port` - Port number, as a string or a number (optional, defaults to 514)

## Flags

- `-idle-timeout` - drop the connection after this much silence from the remote (default `5m`), e.g. `claude mcp add rsh-mcp -- go run github.com/tenox7/rsh-mcp@latest -idle-timeout 15m`

## Notes

**File editing:** rsh-mcp provides read/write via RCP copy in/out. For effective editing, work locally then copy over with upload, or mount workspace via NFS/SMB.

**Privileged port:** Some RSH/RCP servers require source port <1024. This client does that but may require elevated privileges on some systems.

**Windows NT:** If you need an RSH/RCP server for Windows NT check out [NTRSHD](https://github.com/tenox7/ntrshd)

## License

MIT, see [LICENSE](LICENSE).
