// Package rcmd implements the client side of the BSD rcmd/rshd
// protocol shared by rsh and rcp.
package rcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os/user"
	"strings"
	"syscall"
	"time"
)

const dialTimeout = 10 * time.Second

// maxDialAttempts bounds how many reserved source ports we actually try
// to connect through before giving up, so an unreachable host fails in
// reasonable time instead of walking all 512 ports.
const maxDialAttempts = 4

// IdleTimeout bounds silence from the remote, settable via -idle-timeout.
var IdleTimeout = 5 * time.Minute

// Dial connects to rshd from a privileged source port (512-1023), as
// the protocol requires. It tries those ports in a random order and
// moves on whenever one will not bind or its connection does not come
// up, so many sessions at once and quick reconnects do not all pile
// onto the same port and collide in TIME_WAIT. (The old fixed
// 1023->512 scan always reused the same high port, which timed out on
// reconnect and could not run sessions concurrently.) If no reserved
// port works it falls back to an ephemeral port. Reads on the returned
// conn time out after IdleTimeout of silence so MCP calls can not hang
// forever.
func Dial(hostname, port string) (net.Conn, error) {
	addr := net.JoinHostPort(hostname, port)
	d := net.Dialer{Timeout: dialTimeout}

	attempts := 0
	var lastErr error
	for _, off := range rand.Perm(1024 - 512) {
		d.LocalAddr = &net.TCPAddr{Port: 512 + off}
		conn, err := d.Dial("tcp", addr)
		if err == nil {
			return idleConn{conn}, nil
		}
		lastErr = err
		if portUnavailable(err) {
			// Could not bind this reserved port (busy, or not
			// permitted here); another one is free to try at no cost.
			continue
		}
		// The connection itself did not come up. That can be a
		// TIME_WAIT 4-tuple clash (seen as a timeout) or a reserved
		// source port that momentarily collided with another session
		// (seen as a refusal); either way a different source port
		// usually works, so try a few more before giving up. A host
		// that is genuinely down refuses each one instantly, so we
		// still stop quickly after maxDialAttempts.
		attempts++
		if attempts >= maxDialAttempts {
			return nil, err
		}
	}

	// No reserved port worked (e.g. not permitted to bind them here);
	// fall back to an ephemeral port so the client still functions.
	d.LocalAddr = nil
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		if lastErr != nil {
			return nil, fmt.Errorf("no reserved source port available (%v); ephemeral port also failed: %w", lastErr, err)
		}
		return nil, err
	}
	return idleConn{conn}, nil
}

func portUnavailable(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE) ||
		errors.Is(err, syscall.EACCES) ||
		errors.Is(err, syscall.EPERM)
}

type idleConn struct{ net.Conn }

func (c idleConn) Read(p []byte) (int, error) {
	c.SetReadDeadline(time.Now().Add(IdleTimeout))
	return c.Conn.Read(p)
}

// SendRequest sends the rshd handshake: stderr port (none), client
// user, server user, command.
func SendRequest(conn net.Conn, localUser, remoteUser, command string) error {
	req := []byte("0\x00" + localUser + "\x00" + remoteUser + "\x00" + command + "\x00")
	_, err := conn.Write(req)
	return err
}

// ReadAck reads one rcmd status byte: 0 is success, anything else is
// followed by an error message line.
func ReadAck(r *bufio.Reader) error {
	b, err := r.ReadByte()
	if err == io.EOF {
		return errors.New("server closed connection without response (source port or host rejected?)")
	}
	if err != nil {
		return err
	}
	if b == 0 {
		return nil
	}
	msg, _ := r.ReadString('\n')
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return fmt.Errorf("remote error (status %d)", b)
	}
	return errors.New(msg)
}

func LocalUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}
