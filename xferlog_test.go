package main

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tenox7/rsh-mcp/internal/xferlog"
)

func TestTransferLog(t *testing.T) {
	dir := t.TempDir()
	l, err := xferlog.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	transfers = l
	defer func() { transfers = nil }()
	port, _ := fakeRshd(t, func(_ rcmdRequest, _ *bufio.Reader, conn net.Conn) {
		conn.Write([]byte("abc"))
	})
	cs := connect(t)
	call(t, cs, "exec", map[string]any{"host": "127.0.0.1", "port": port, "user": "u", "command": "x", "shell": "login"})
	b, err := os.ReadFile(filepath.Join(dir, xferlog.FileName))
	if err != nil {
		t.Fatal(err)
	}
	line := string(b)
	for _, want := range []string{" exec ok host=127.0.0.1 port=" + port + " user=u ", ` command="x" `, " bytes=4 ",
		" sha256=" + xferlog.Sum([]byte("abc\n")), " exit=unknown "} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q lacks %q", line, want)
		}
	}
}
