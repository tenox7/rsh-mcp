package main

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestPortStringOrNumber(t *testing.T) {
	port, _ := fakeRshd(t, func(_ rcmdRequest, _ *bufio.Reader, conn net.Conn) {
		conn.Write([]byte("hi\n"))
	})
	n, _ := strconv.Atoi(port)
	cs := connect(t)
	for _, p := range []any{port, n} {
		out, isErr := call(t, cs, "exec", map[string]any{"host": "127.0.0.1", "port": p, "user": "u", "command": "x", "shell": "login"})
		if isErr || !strings.HasPrefix(out, "hi\n") {
			t.Errorf("port %#v: error %v, %q", p, isErr, out)
		}
	}
}

func TestPortString(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want string
	}{
		{nil, "514"}, {"", "514"}, {"5141", "5141"}, {float64(5141), "5141"},
	} {
		if got := portString(tc.in); got != tc.want {
			t.Errorf("portString(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
