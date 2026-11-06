package rsh

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tenox7/rsh-mcp/internal/rcmd"
)

const maxCapture = 32 << 20

// scriptCommand starts /bin/sh on the script. ntrshd 1.2 and later on
// Windows NT know the rsh-mcp-1 tag and run the script's command with cmd.
const scriptCommand = "sh -s rsh-mcp-1"

const cmdExeNotFound = "internal or external command"

type Options struct {
	Host     string
	User     string
	Port     string
	Command  string
	Shell    string
	Timeout  time.Duration
	Grep     string
	MaxLines int
	MaxBytes int
	Tail     bool
}

type Result struct {
	Output     []byte
	ExitStatus int
	TimedOut   bool
	Lines      int
	Matched    int
	Shown      int
	Cut        bool
}

const ExitUnknown = -1

func Execute(ctx context.Context, o Options) (*Result, error) {
	grep, err := compileGrep(o.Grep)
	if err != nil {
		return nil, err
	}
	localUser, err := rcmd.LocalUser()
	if err != nil {
		return nil, err
	}
	if o.User == "" {
		o.User = localUser
	}

	conn, err := rcmd.Dial(o.Host, o.Port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	viaSh := o.Shell != "login"
	command := o.Command
	if viaSh {
		command = scriptCommand
	}
	if err := rcmd.SendRequest(conn, localUser, o.User, command); err != nil {
		return nil, err
	}
	r := bufio.NewReader(conn)
	if err := rcmd.ReadAck(r); err != nil {
		return nil, fmt.Errorf("rsh %s: %w", o.Host, err)
	}

	marker := newMarker()
	if viaSh {
		if _, err := io.WriteString(conn, script(o.Command, marker)); err != nil {
			return nil, err
		}
	}

	stop := watch(ctx, conn, o.Timeout)
	output, _ := io.ReadAll(io.LimitReader(r, maxCapture))
	timedOut := stop()
	if ctx.Err() != nil {
		return nil, errors.New("cancelled")
	}
	output = bytes.ReplaceAll(output, []byte("\r\n"), []byte("\n"))

	res := &Result{ExitStatus: ExitUnknown, TimedOut: timedOut}
	if viaSh {
		output, res.ExitStatus = cutMarker(output, marker)
	}
	if viaSh && res.ExitStatus == ExitUnknown && !timedOut && bytes.Contains(output, []byte(cmdExeNotFound)) {
		return nil, fmt.Errorf("%s has no sh: it answered like Windows NT's cmd.exe, so the command did not run. "+
			"Run ntrshd 1.2 or later there (github.com/tenox7/ntrshd), or use shell \"login\" (no exit status)", o.Host)
	}
	res.Output, res.Lines, res.Matched, res.Shown, res.Cut = limit(output, grep, o.MaxLines, o.MaxBytes, o.Tail)
	return res, nil
}

// The command runs in a subshell of /bin/sh that reads this script from
// stdin, so the login shell (sh, ksh, csh) never parses it: quotes, !,
// comments and newlines all work, and the exit status comes back. shift
// drops the tag that scriptCommand gives sh.
func script(command, marker string) string {
	return "shift\n(\n" + command + "\n) </dev/null 2>&1\necho " + marker + "$?__\nexit\n"
}

func newMarker() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "__rshmcp_" + hex.EncodeToString(b) + "_"
}

func cutMarker(output []byte, marker string) ([]byte, int) {
	at := bytes.LastIndex(output, []byte(marker))
	if at < 0 {
		return output, ExitUnknown
	}
	digits := output[at+len(marker):]
	end := bytes.Index(digits, []byte("__"))
	if end < 0 {
		return output, ExitUnknown
	}
	status, err := strconv.Atoi(string(digits[:end]))
	if err != nil {
		return output, ExitUnknown
	}
	return output[:at], status
}

func watch(ctx context.Context, conn io.Closer, timeout time.Duration) func() bool {
	var fired atomic.Bool
	stopCtx := context.AfterFunc(ctx, func() { conn.Close() })
	if timeout <= 0 {
		return func() bool { stopCtx(); return false }
	}
	timer := time.AfterFunc(timeout, func() {
		fired.Store(true)
		conn.Close()
	})
	return func() bool {
		stopCtx()
		timer.Stop()
		return fired.Load()
	}
}

func compileGrep(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}
	return re, nil
}

func limit(output []byte, grep *regexp.Regexp, maxLines, maxBytes int, tail bool) ([]byte, int, int, int, bool) {
	text := strings.TrimSuffix(string(output), "\n")
	if text == "" {
		return nil, 0, 0, 0, false
	}
	lines := strings.Split(text, "\n")
	total := len(lines)
	if grep != nil {
		lines = keep(lines, grep)
	}
	matched := len(lines)

	cut := false
	if maxLines > 0 && len(lines) > maxLines {
		lines = pick(lines, maxLines, tail)
		cut = true
	}
	out := strings.Join(lines, "\n")
	if maxBytes > 0 && len(out) > maxBytes {
		out = cutBytes(out, maxBytes, tail)
		cut = true
	}
	shown := strings.Count(out, "\n") + 1
	return []byte(out + "\n"), total, matched, shown, cut
}

func keep(lines []string, re *regexp.Regexp) []string {
	var kept []string
	for _, l := range lines {
		if !re.MatchString(l) {
			continue
		}
		kept = append(kept, l)
	}
	return kept
}

func pick(lines []string, n int, tail bool) []string {
	if tail {
		return lines[len(lines)-n:]
	}
	return lines[:n]
}

func cutBytes(s string, n int, tail bool) string {
	if tail {
		s = s[len(s)-n:]
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			return s[i+1:]
		}
		return s
	}
	s = s[:n]
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func ParseUserHost(userHost string) (username, hostname string, err error) {
	parts := strings.Split(userHost, "@")

	switch len(parts) {
	case 1:
		username, err = rcmd.LocalUser()
		if err != nil {
			return "", "", err
		}
		hostname = parts[0]
	case 2:
		username = parts[0]
		hostname = parts[1]
	default:
		return "", "", errors.New("invalid username@hostname format")
	}

	return username, hostname, nil
}
