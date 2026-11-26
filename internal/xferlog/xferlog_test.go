package xferlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOff(t *testing.T) {
	l, err := Open("")
	if l != nil || err != nil {
		t.Fatalf("Open(\"\") = %v, %v; want a nil logger", l, err)
	}
	l.Log(Entry{Op: "exec"})
}

func TestFormat(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := format(at, Entry{Op: "download", Host: "h", Port: "514", User: "u", LocalUser: "l",
		Paths: []string{"a b"}, To: "/d", Bytes: 3, Duration: 1500 * time.Millisecond,
		Files: []File{{"/d/a b", 3, Sum([]byte("abc"))}}})
	want := `2026-01-02T03:04:05Z download ok host=h port=514 user=u local_user=l path="a b" to="/d" bytes=3 ms=1500` + "\n" +
		"\tfile=\"/d/a b\" bytes=3 sha256=ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	got = format(at, Entry{Op: "exec", Command: "ls\n-l", Exit: "1", Err: "boom"})
	if !strings.Contains(got, ` exec failed `) || !strings.Contains(got, `command="ls\n-l"`) ||
		!strings.Contains(got, ` exit=1 `) || !strings.HasSuffix(got, " error=\"boom\"\n") {
		t.Errorf("got %q", got)
	}
}

func TestAppends(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	l.Log(Entry{Op: "read"})
	l.Log(Entry{Op: "write"})
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), "\n"); n != 2 {
		t.Errorf("%d lines, want 2:\n%s", n, b)
	}
}
