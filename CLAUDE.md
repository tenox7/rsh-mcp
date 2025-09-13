You are writing an MCP server for RSH / RCP (remote shell, remote copy the pre-SSH protocol, for old computers)

in priorart folder you will find example ssh-mcp server however please DO NOT take it as an example of a good MCP server, just a reference because it's similar to what we're doing, excep ssh we use rsh. but it's not example of good mcp, design it by the book

in ../rsh folder there are rsh/rcp clients in go which you wrote which I typicaly use, we need to include it in the project

The mcp server will expose two tools:

"exec" - that will execute command remotely via rsh client
"copy" - or rather perhaps "read" and "write" that will allow to read/write files using rcp client

Note that rsh server will not require any authentication (password or otherwise)

the mcp server must be written in go and fully self contained\

in readme.md provide example of how to include it in claude code / desktop config

go style guide:
- keep indent to the left
- avoid nesting
- use early return/continue/break on !=
- instead of comments write readable code
