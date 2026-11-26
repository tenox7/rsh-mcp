package rcmd

import (
	"bufio"
	"errors"
	"net"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// unprivileged skips the test where this process may bind reserved ports.
func unprivileged(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("reserved-port binding policy differs on macOS")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:512")
	if err == nil {
		ln.Close()
		t.Skip("this process may bind reserved ports")
	}
	if !errors.Is(err, syscall.EACCES) && !errors.Is(err, syscall.EPERM) {
		t.Skip("cannot tell whether reserved ports bind here")
	}
}

// hangUp listens like an rshd that drops clients on unprivileged ports.
func hangUp(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func TestUnprivilegedRejectionExplained(t *testing.T) {
	unprivileged(t)
	conn, err := Dial("127.0.0.1", hangUp(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	err = ReadAnswer(conn, bufio.NewReader(conn))
	if !errors.Is(err, ErrNoResponse) || !strings.Contains(err.Error(), "setcap cap_net_bind_service=+ep") {
		t.Errorf("error %v, want the setcap hint", err)

	}
}

func TestRequirePrivilegedPort(t *testing.T) {
	unprivileged(t)
	RequirePrivilegedPort = true
	defer func() { RequirePrivilegedPort = false }()
	conn, err := Dial("127.0.0.1", hangUp(t))
	if err == nil {
		conn.Close()
		t.Fatal("Dial used an unprivileged port")
	}
	if !strings.Contains(err.Error(), "setcap cap_net_bind_service=+ep") {
		t.Errorf("error %v, want the setcap hint", err)

	}
}
