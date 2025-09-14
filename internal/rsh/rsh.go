package rsh

import (
	"errors"
	"fmt"
	"net"
	"os/user"
	"strings"
	"time"
)

func Execute(hostname, username, command, port string) ([]byte, error) {
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

	var result []byte
	buffer := make([]byte, 4096)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}
		result = append(result, buffer[:n]...)
	}

	return result, nil
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