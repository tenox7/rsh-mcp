// Package xferlog appends one line per rsh-mcp operation to
// rsh_transfers.log, for an audit trail of what ran and what was copied.
package xferlog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const FileName = "rsh_transfers.log"

type File struct {
	Path   string
	Size   int64
	SHA256 string
}

type Entry struct {
	Op        string
	Host      string
	Port      string
	User      string
	LocalUser string
	Command   string
	Paths     []string
	To        string
	Bytes     int64
	SHA256    string
	Files     []File
	Exit      string
	Err       string
	Duration  time.Duration
}

type Logger struct {
	mu   sync.Mutex
	path string
}

// Open returns a logger writing to dir/rsh_transfers.log, or nil (which
// logs nothing) when dir is empty.
func Open(dir string) (*Logger, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Logger{path: filepath.Join(dir, FileName)}, nil
}

func (l *Logger) Log(e Entry) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("xferlog: %v", err)
		return
	}
	defer f.Close()
	if _, err := io.WriteString(f, format(time.Now(), e)); err != nil {
		log.Printf("xferlog: %v", err)
	}
}

func format(t time.Time, e Entry) string {
	var b strings.Builder
	status := "ok"
	if e.Err != "" {
		status = "failed"
	}
	fmt.Fprintf(&b, "%s %s %s host=%s port=%s user=%s local_user=%s",
		t.UTC().Format(time.RFC3339Nano), e.Op, status, e.Host, e.Port, e.User, e.LocalUser)
	if e.Command != "" {
		fmt.Fprintf(&b, " command=%s", strconv.Quote(e.Command))
	}
	for _, p := range e.Paths {
		fmt.Fprintf(&b, " path=%s", strconv.Quote(p))
	}
	if e.To != "" {
		fmt.Fprintf(&b, " to=%s", strconv.Quote(e.To))
	}
	fmt.Fprintf(&b, " bytes=%d", e.Bytes)
	if e.SHA256 != "" {
		fmt.Fprintf(&b, " sha256=%s", e.SHA256)
	}
	if e.Exit != "" {
		fmt.Fprintf(&b, " exit=%s", e.Exit)
	}
	fmt.Fprintf(&b, " ms=%d", e.Duration.Milliseconds())
	if e.Err != "" {
		fmt.Fprintf(&b, " error=%s", strconv.Quote(e.Err))
	}
	b.WriteString("\n")
	for _, f := range e.Files {
		fmt.Fprintf(&b, "\tfile=%s bytes=%d sha256=%s\n", strconv.Quote(f.Path), f.Size, f.SHA256)
	}
	return b.String()
}

func Sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// SumFile hashes a local file, or returns "unknown" if it can not.
func SumFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(h.Sum(nil))
}
