package rsh

import (
	"errors"
	"net"
	"os/user"
	"strings"
)

func Execute(hostname, username, command, port string) ([]byte, error) {
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, err
		}
		username = currentUser.Username
	}

	conn, err := net.Dial("tcp", hostname+":"+port)
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