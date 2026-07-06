// Package rcmd implements the client side of the BSD rcmd/rshd
// protocol shared by rsh and rcp.
package rcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os/user"
	"strings"
	"syscall"
	"time"
)

const (
	dialTimeout = 10 * time.Second
	idleTimeout = 30 * time.Second
)

// Dial prefers privileged source ports 512-1023 as rshd requires,
// falling back to an ephemeral port if none can be bound. Reads on
// the returned conn time out after idleTimeout of silence so MCP
// calls can not hang forever.
func Dial(hostname, port string) (net.Conn, error) {
	addr := net.JoinHostPort(hostname, port)
	d := net.Dialer{Timeout: dialTimeout}
	for p := 1023; p >= 512; p-- {
		d.LocalAddr = &net.TCPAddr{Port: p}
		conn, err := d.Dial("tcp", addr)
		if err == nil {
			return idleConn{conn}, nil
		}
		if portUnavailable(err) {
			continue
		}
		return nil, err
	}
	d.LocalAddr = nil
	conn, err := d.Dial("tcp", addr)
	if err != nil {
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
	c.SetReadDeadline(time.Now().Add(idleTimeout))
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
