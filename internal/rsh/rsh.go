package rsh

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/tenox7/rsh-mcp/internal/rcmd"
)

func Execute(hostname, username, command, port string, maxLines, maxBytes int, tail bool) ([]byte, error) {
	localUser, err := rcmd.LocalUser()
	if err != nil {
		return nil, err
	}
	if username == "" {
		username = localUser
	}

	conn, err := rcmd.Dial(hostname, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := rcmd.SendRequest(conn, localUser, username, command); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	if err := rcmd.ReadAck(r); err != nil {
		return nil, fmt.Errorf("rsh %s: %w", hostname, err)
	}

	if maxLines <= 0 {
		maxLines = 1000
	}
	if maxBytes <= 0 {
		maxBytes = 100000
	}

	var result []byte
	buffer := make([]byte, 4096)
	for len(result) < maxBytes {
		n, err := r.Read(buffer)
		if n > maxBytes-len(result) {
			n = maxBytes - len(result)
		}
		result = append(result, buffer[:n]...)
		if err != nil {
			break
		}
	}

	lines := bytes.Split(result, []byte{'\n'})
	if len(lines) > maxLines {
		if tail {
			lines = lines[len(lines)-maxLines:]
		} else {
			lines = lines[:maxLines]
		}
		return bytes.Join(lines, []byte{'\n'}), nil
	}

	return result, nil
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
