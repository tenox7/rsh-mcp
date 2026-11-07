package main

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"testing"
)

type rcmdRequest struct {
	LocalUser  string
	RemoteUser string
	Command    string
}

// fakeRshd plays rshd on a local port: for each connection it reads the
// rcmd handshake, reports it on reqs, acks it and lets serve answer.
func fakeRshd(t *testing.T, serve func(req rcmdRequest, r *bufio.Reader, conn net.Conn)) (port string, reqs chan rcmdRequest) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	reqs = make(chan rcmdRequest, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			r := bufio.NewReader(conn)
			var f [4]string
			for i := range f {
				s, _ := r.ReadString(0)
				f[i] = strings.TrimSuffix(s, "\x00")
			}
			req := rcmdRequest{f[1], f[2], f[3]}
			reqs <- req
			conn.Write([]byte{0})
			serve(req, r, conn)
			conn.Close()
		}
	}()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port), reqs
}
