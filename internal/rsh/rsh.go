package rsh

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os/user"
	"strings"
	"time"
)

func Execute(hostname, username, command, port string, maxLines, maxBytes int, tail bool) ([]byte, error) {
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, err
		}
		username = currentUser.Username
	}

	// Connect from privileged port (RSH protocol requirement)
	conn, err := connectFromPrivilegedPort(hostname, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	localUser := currentUser.Username

	data := []byte{0}
	data = append(data, []byte(localUser)...)
	data = append(data, 0)
	data = append(data, []byte(username)...)
	data = append(data, 0)
	data = append(data, []byte(command)...)
	data = append(data, 0)

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
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
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}
		remaining := maxBytes - len(result)
		if n > remaining {
			result = append(result, buffer[:remaining]...)
			break
		}
		result = append(result, buffer[:n]...)
	}

	if len(result) == 0 {
		return result, nil
	}

	lines := bytes.Split(result, []byte{'\n'})
	if len(lines) > maxLines {
		if tail {
			lines = lines[len(lines)-maxLines:]
		} else {
			lines = lines[:maxLines]
		}
	}

	return bytes.Join(lines, []byte{'\n'}), nil
}

func ParseUserHost(userHost string) (username, hostname string, err error) {
	parts := strings.Split(userHost, "@")

	switch len(parts) {
	case 1:
		currentUser, err := user.Current()
		if err != nil {
			return "", "", err
		}
		username = currentUser.Username
		hostname = parts[0]
	case 2:
		username = parts[0]
		hostname = parts[1]
	default:
		return "", "", errors.New("invalid username@hostname format")
	}

	return username, hostname, nil
}

// connectFromPrivilegedPort connects to a remote host from a privileged port (512-1023)
// This is required by the RSH protocol for authentication
func connectFromPrivilegedPort(hostname, port string) (net.Conn, error) {
	// Try to bind to a privileged port
	for localPort := 1023; localPort >= 512; localPort-- {
		localAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", localPort))
		if err != nil {
			continue
		}

		remoteAddr, err := net.ResolveTCPAddr("tcp", hostname+":"+port)
		if err != nil {
			return nil, err
		}

		conn, err := net.DialTCP("tcp", localAddr, remoteAddr)
		if err == nil {
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			return conn, nil
		}
	}

	return nil, fmt.Errorf("could not connect from privileged port (need appropriate privileges)")
}