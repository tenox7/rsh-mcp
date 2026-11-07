package rsh

import (
	"bufio"
	"context"
	"net"
	"testing"
)

func TestLineEnds(t *testing.T) {
	port, _ := fakeRshd(t, func(_ rcmdRequest, _ *bufio.Reader, conn net.Conn) {
		conn.Write([]byte("a\r\nb\r\r\nc\rd\n\ne\r"))
	})
	res, err := Execute(context.Background(), Options{Host: "127.0.0.1", Port: port, User: "u", Command: "x", Shell: "login"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "a\nb\nc\rd\n\ne\n"; string(res.Output) != want {
		t.Errorf("output %q, want %q", res.Output, want)
	}
}
