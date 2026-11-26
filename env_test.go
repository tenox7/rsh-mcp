package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvDefaults(t *testing.T) {
	port, reqs := fakeRshd(t, func(_ rcmdRequest, _ *bufio.Reader, conn net.Conn) {
		conn.Write([]byte("hi\n"))
	})
	t.Setenv("RSH_HOST", "127.0.0.1")
	t.Setenv("RSH_PORT", port)
	t.Setenv("RSH_USER", "u")
	t.Setenv("RSH_LOCAL_USER", "l")
	t.Setenv("RSH_SHELL", "login")
	cs := connect(t)

	out, isErr := call(t, cs, "exec", map[string]any{"command": "SHOW TIME"})
	if isErr || out != "hi\n" {
		t.Errorf("exec: error %v, %q", isErr, out)
	}
	if req := <-reqs; req != (rcmdRequest{"l", "u", "SHOW TIME"}) {
		t.Errorf("handshake %+v", req)
	}

	call(t, cs, "exec", map[string]any{"command": "x", "user": "u2", "local_user": "l2"})
	if req := <-reqs; req.LocalUser != "l2" || req.RemoteUser != "u2" {
		t.Errorf("arguments did not override the environment: %+v", req)
	}
}

func TestLocalUserOnCopy(t *testing.T) {
	port, reqs := fakeRshd(t, func(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
		r.ReadByte()
		io.WriteString(conn, "C0644 3 a.txt\n")
		r.ReadByte()
		io.WriteString(conn, "abc\x00")
		r.ReadByte()
	})
	dir := t.TempDir()
	cs := connect(t)
	out, isErr := call(t, cs, "download", map[string]any{"host": "127.0.0.1", "port": port,
		"user": "u", "local_user": "l", "paths": []string{"a.txt"}, "to": dir})
	if isErr {
		t.Fatal(out)
	}
	if req := <-reqs; req.LocalUser != "l" || req.RemoteUser != "u" {
		t.Errorf("handshake %+v", req)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "abc" {
		t.Errorf("file holds %q", b)
	}
}

func TestHostRequired(t *testing.T) {
	t.Setenv("RSH_HOST", "")
	cs := connect(t)
	out, isErr := call(t, cs, "exec", map[string]any{"command": "x"})
	if !isErr || !strings.Contains(out, "RSH_HOST") {
		t.Errorf("no host: error %v, %q", isErr, out)
	}
}
