package rcp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func ReadFile(hostname, username, remotePath, port string) ([]byte, error) {
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

	command := "rcp -f " + remotePath
	data := []byte{0}
	data = append(data, []byte(currentUser.Username)...)
	data = append(data, 0)
	data = append(data, []byte(username)...)
	data = append(data, 0)
	data = append(data, []byte(command)...)
	data = append(data, 0)

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)

	ack, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("error reading initial response: %v", err)
	}
	if ack != 0 {
		return nil, errors.New("remote file not found or access denied")
	}

	metadataLine, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("error reading file metadata: %v", err)
	}

	metadataStr := strings.TrimSpace(string(metadataLine))
	if !strings.HasPrefix(metadataStr, "C") {
		return nil, fmt.Errorf("unexpected metadata format: %s", metadataStr)
	}

	parts := strings.SplitN(metadataStr[1:], " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid metadata format: %s", metadataStr)
	}

	fileSize, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing file size: %v", err)
	}

	_, err = conn.Write([]byte{0})
	if err != nil {
		return nil, fmt.Errorf("error sending metadata acknowledgement: %v", err)
	}

	var content []byte
	buffer := make([]byte, 32*1024)
	var totalReceived int64
	for totalReceived < fileSize {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading file content: %v", err)
		}
		if n > 0 {
			content = append(content, buffer[:n]...)
			totalReceived += int64(n)
		}
		if err == io.EOF {
			break
		}
	}

	endMarker, err := reader.ReadByte()
	if err != nil || endMarker != 0 {
		return nil, fmt.Errorf("unexpected end-of-file marker: %v", err)
	}

	_, err = conn.Write([]byte{0})
	if err != nil {
		return nil, fmt.Errorf("error sending final acknowledgement: %v", err)
	}

	return content, nil
}

func WriteFile(hostname, username, remotePath, port string, content []byte) error {
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return err
		}
		username = currentUser.Username
	}

	conn, err := net.Dial("tcp", hostname+":"+port)
	if err != nil {
		return err
	}
	defer conn.Close()

	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	command := "rcp -t " + remotePath
	data := []byte{0}
	data = append(data, []byte(currentUser.Username)...)
	data = append(data, 0)
	data = append(data, []byte(username)...)
	data = append(data, 0)
	data = append(data, []byte(command)...)
	data = append(data, 0)

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("error sending command: %v", err)
	}

	reader := bufio.NewReader(conn)

	ack1, err := reader.ReadByte()
	if err != nil || ack1 != 0 {
		return fmt.Errorf("failed to receive first acknowledgement: %v", err)
	}

	ack2, err := reader.ReadByte()
	if err != nil || ack2 != 0 {
		return fmt.Errorf("failed to receive second acknowledgement: %v", err)
	}

	fileName := filepath.Base(remotePath)
	fileMode := 0644
	fileSize := len(content)
	fileInfoStr := fmt.Sprintf("C%04o %d %s\n", fileMode, fileSize, fileName)
	_, err = conn.Write([]byte(fileInfoStr))
	if err != nil {
		return fmt.Errorf("error sending file info: %v", err)
	}

	ack, err := reader.ReadByte()
	if err != nil || ack != 0 {
		return fmt.Errorf("failed to receive file info acknowledgement: %v", err)
	}

	_, err = conn.Write(content)
	if err != nil {
		return fmt.Errorf("error sending file content: %v", err)
	}

	_, err = conn.Write([]byte{0})
	if err != nil {
		return fmt.Errorf("error sending end-of-file marker: %v", err)
	}

	ack, err = reader.ReadByte()
	if err != nil || ack != 0 {
		return fmt.Errorf("failed to receive final acknowledgement: %v", err)
	}

	_, err = conn.Write([]byte("E\n"))
	if err != nil {
		return fmt.Errorf("error sending end command: %v", err)
	}

	ack, err = reader.ReadByte()
	if err != nil || ack != 0 {
		return fmt.Errorf("failed to receive end acknowledgement: %v", err)
	}

	return nil
}