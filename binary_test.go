package main

import (
	"bufio"
	"encoding/base64"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
)

var binary = []byte{0, 1, 0xff, 0xfe, '\r', '\n', 0x80}

func serveBinary(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
	r.ReadByte()
	io.WriteString(conn, "C0644 "+strconv.Itoa(len(binary))+" b.bin\n")
	r.ReadByte()
	conn.Write(append(binary, 0))
	r.ReadByte()
}

func TestReadBase64(t *testing.T) {
	port, _ := fakeRshd(t, serveBinary)
	cs := connect(t)
	out, isErr := call(t, cs, "read", map[string]any{"host": "127.0.0.1", "port": port, "user": "u", "path": "b.bin", "encoding": "base64"})
	if isErr || out != base64.StdEncoding.EncodeToString(binary) {
		t.Errorf("error %v, %q", isErr, out)
	}
}

func TestReadTextRefusesBinary(t *testing.T) {
	port, _ := fakeRshd(t, serveBinary)
	cs := connect(t)
	out, isErr := call(t, cs, "read", map[string]any{"host": "127.0.0.1", "port": port, "user": "u", "path": "b.bin"})
	if !isErr || !strings.Contains(out, "encoding base64") {
		t.Errorf("error %v, %q", isErr, out)
	}
}

func TestWriteBase64AndMode(t *testing.T) {
	type got struct {
		header string
		data   []byte
	}
	recv := make(chan got, 1)
	port, _ := fakeRshd(t, func(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
		conn.Write([]byte{0})
		header, _ := r.ReadString('\n')
		conn.Write([]byte{0})
		data := make([]byte, len(binary)+1)
		io.ReadFull(r, data)
		conn.Write([]byte{0})
		r.ReadString('\n')
		conn.Write([]byte{0})
		recv <- got{header, data[:len(binary)]}
	})
	cs := connect(t)
	out, isErr := call(t, cs, "write", map[string]any{"host": "127.0.0.1", "port": port, "user": "u", "path": "x/b.bin",
		"content_base64": base64.StdEncoding.EncodeToString(binary), "mode": "0755"})
	if isErr {
		t.Fatal(out)
	}
	g := <-recv
	if g.header != "C0755 7 b.bin\n" || string(g.data) != string(binary) {
		t.Errorf("remote got %q %q", g.header, g.data)
	}
}

func TestWriteArgumentErrors(t *testing.T) {
	cs := connect(t)
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"content": "a", "content_base64": "YQ=="}, "not both"},
		{map[string]any{"content_base64": "not base64!"}, "content_base64"},
		{map[string]any{"content": "a", "mode": "rw-r--r--"}, "mode"},
		{map[string]any{"content": "a", "mode": "1777"}, "mode"},
	} {
		tc.args["host"], tc.args["port"], tc.args["path"] = "127.0.0.1", "1", "f"
		if out, isErr := call(t, cs, "write", tc.args); !isErr || !strings.Contains(out, tc.want) {
			t.Errorf("%v: error %v, %q", tc.args, isErr, out)
		}
	}
}
