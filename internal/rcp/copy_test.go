package rcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModeFlagComesLastForOldRcpThatStartsOnIt(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{sourceCommand("/etc/hosts", false), "rcp -f /etc/hosts"},
		{sourceCommand("/usr/a", true), "rcp -r -f /usr/a"},
		{sinkCommand("/tmp", false, false), "rcp -t /tmp"},
		{sinkCommand("/tmp", true, false), "rcp -d -t /tmp"},
		{sinkCommand("/tmp", true, true), "rcp -r -d -t /tmp"},
		{sinkCommand("/tmp/my dir", true, false), "rcp -d -t '/tmp/my dir'"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestWindowsPathGoesAsItIsForNTHasNoShellToTakeQuotesOff(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{sourceCommand(`C:\WINNT35\System32\ntrshd.log`, false), `rcp -f C:\WINNT35\System32\ntrshd.log`},
		{sourceCommand(`C:\Program Files\read me.txt`, false), `rcp -f C:\Program Files\read me.txt`},
		{sourceCommand(`C:/Program Files/read me.txt`, false), `rcp -f C:/Program Files/read me.txt`},
		{sourceCommand(`temp\b\ntrshd.exe`, false), `rcp -f temp\b\ntrshd.exe`},
		{sinkCommand(`D:\temp`, true, false), `rcp -d -t D:\temp`},
		{sourceCommand("/tmp/read me.txt", false), "rcp -f '/tmp/read me.txt'"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestDownloadAsksForEachPathOnItsOwnForNTTakesTheRestOfTheLineAsOnePath(t *testing.T) {
	port := fakeNtrshd(t)
	dir := t.TempDir()
	want := map[string]string{
		"ntrshd.log":  `C:\WINNT35\System32\ntrshd.log`,
		"read me.txt": `C:\Program Files\read me.txt`,
	}
	paths := []string{want["ntrshd.log"], want["read me.txt"]}

	rep, err := Download(context.Background(), "127.0.0.1", "", port, paths, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Errors) > 0 {
		t.Fatal(rep.Errors)
	}
	for name, path := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != path {
			t.Errorf("ntrshd was asked for %q, want %q", got, path)
		}
	}
}

// fakeNtrshd serves "rcp -f" as ntrshd does: the whole rest of the line is
// one path, quotes and all. The file it sends holds that path.
func fakeNtrshd(t *testing.T) string {
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
			sendPathAsFile(conn)
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return port
}

func sendPathAsFile(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	var command string
	for range 4 {
		field, err := r.ReadString(0)
		if err != nil {
			return
		}
		command = strings.TrimSuffix(field, "\x00")
	}
	path := strings.TrimPrefix(command, "rcp -f ")
	name := path[strings.LastIndexAny(path, `\/`)+1:]
	conn.Write([]byte{0})
	r.ReadByte()
	fmt.Fprintf(conn, "C0644 %d %s\n", len(path), name)
	r.ReadByte()
	io.WriteString(conn, path+"\x00")
	r.ReadByte()
}
