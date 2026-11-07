package rcp

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// serveFile plays "rcp -f" for one file called name.
func serveFile(name, content string) func(rcmdRequest, *bufio.Reader, net.Conn) {
	return func(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
		r.ReadByte()
		io.WriteString(conn, "C0644 "+strconv.Itoa(len(content))+" "+name+"\n")
		r.ReadByte()
		io.WriteString(conn, content+"\x00")
		r.ReadByte()
	}
}

// sinkFile plays "rcp -t" for one file.
func sinkFile(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
	conn.Write([]byte{0})
	line, _ := r.ReadString('\n')
	size, _ := strconv.Atoi(strings.Fields(line)[1])
	conn.Write([]byte{0})
	io.CopyN(io.Discard, r, int64(size)+1)
	conn.Write([]byte{0})
}

func commands(reqs chan rcmdRequest, n int) []string {
	var got []string
	for range n {
		got = append(got, (<-reqs).Command)
	}
	return got
}

func TestDownloadQuoting(t *testing.T) {
	const spec = "SYS$SYSDEVICE:[HOME.U]A.TXT;1"
	for _, tc := range []struct {
		login bool
		want  string
	}{
		{false, "rcp -f '" + spec + "'"},
		{true, "rcp -f " + spec},
	} {
		port, reqs := fakeRshd(t, serveFile("A.TXT", "abc"))
		dir := t.TempDir()
		rep, err := Download(context.Background(), "127.0.0.1", "u", port, []string{spec}, dir, false, tc.login)
		if err != nil || len(rep.Files) != 1 {
			t.Fatalf("login %v: %v %+v", tc.login, err, rep)
		}
		if got := (<-reqs).Command; got != tc.want {
			t.Errorf("login %v sent %q, want %q", tc.login, got, tc.want)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "A.TXT")); string(b) != "abc" {
			t.Errorf("login %v: file holds %q", tc.login, b)
		}
	}
}

func TestDownloadLoginOneSessionPerPath(t *testing.T) {
	port, reqs := fakeRshd(t, serveFile("A.TXT", "abc"))
	rep, err := Download(context.Background(), "127.0.0.1", "u", port, []string{"[D]A.TXT", "[D]*.COM"}, t.TempDir(), false, true)
	if err != nil || len(rep.Files) != 2 {
		t.Fatalf("%v %+v", err, rep)
	}
	got := commands(reqs, 2)
	if got[0] != "rcp -f [D]A.TXT" || got[1] != "rcp -f [D]*.COM" {
		t.Errorf("sent %q", got)
	}
}

func TestUploadLoginOneSessionPerFile(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, n := range []string{"a.txt", "b.txt"} {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte(n), 0644)
		paths = append(paths, p)
	}
	for _, tc := range []struct {
		to    string
		paths []string
		want  []string
	}{
		{"[D]", paths, []string{"rcp -t [D]a.txt", "rcp -t [D]b.txt"}},
		{"DEV:", paths[:1], []string{"rcp -t DEV:a.txt"}},
		{"d", paths, []string{"rcp -t d/a.txt", "rcp -t d/b.txt"}},
		{"[D]NEW.TXT;", paths[:1], []string{"rcp -t [D]NEW.TXT;"}},
	} {
		port, reqs := fakeRshd(t, sinkFile)
		rep, err := Upload(context.Background(), "127.0.0.1", "u", port, tc.paths, tc.to, false, true)
		if err != nil || len(rep.Files) != len(tc.paths) || len(rep.Errors) > 0 {
			t.Fatalf("to %s: %v %+v", tc.to, err, rep)
		}
		got := commands(reqs, len(tc.want))
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("to %s: sent %q, want %q", tc.to, got, tc.want)
		}
	}
}

func TestUploadShQuoting(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("a"), 0644)
	port, reqs := fakeRshd(t, sinkFile)
	if _, err := Upload(context.Background(), "127.0.0.1", "u", port, []string{p}, "my dir", false, false); err != nil {
		t.Fatal(err)
	}
	if got := (<-reqs).Command; got != "rcp -t 'my dir'" {
		t.Errorf("sent %q", got)
	}
}
