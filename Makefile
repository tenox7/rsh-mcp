# make install builds rsh-mcp and lets it bind the reserved source ports
# (512-1023) that most rshd servers insist on. A rebuild drops that right,
# so run make install again after each one.

BIN ?= $(shell go env GOPATH)/bin/rsh-mcp

.PHONY: build test install

build:
	go build -o rsh-mcp .

test:
	go test ./...

install:
	mkdir -p $(dir $(BIN))
	go build -o $(BIN) .
	@echo
	@echo 'Most rshd servers only answer clients on a source port below 1024.'
	@echo 'Binding one needs root or the cap_net_bind_service capability, so'
	@echo 'this gives $(BIN) that one capability. sudo may ask for your password:'
	@echo '  sudo setcap cap_net_bind_service=+ep $(BIN)'
	@echo
	sudo setcap cap_net_bind_service=+ep $(BIN)
	@getcap $(BIN)
