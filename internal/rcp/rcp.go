package rcp

import (
	"bufio"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tenox7/rsh-mcp/internal/rcmd"
)

func ReadFile(hostname, username, remotePath, port string) ([]byte, error) {
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

	err = rcmd.SendRequest(conn, localUser, username, "rcp -f "+remotePath)
	if err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	if err := rcmd.ReadAck(r); err != nil {
		return nil, fmt.Errorf("rsh %s: %w", hostname, err)
	}

	// receiver announces readiness, source answers with one control
	// line per file: Cmode size name
	if _, err := conn.Write([]byte{0}); err != nil {
		return nil, err
	}

	line, err := r.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading file info: %w", err)
	}
	if line[0] == 1 || line[0] == 2 {
		return nil, errors.New(strings.TrimSpace(line[1:]))
	}
	if line[0] != 'C' {
		return nil, fmt.Errorf("unexpected rcp control message: %q", strings.TrimSpace(line))
	}

	parts := strings.SplitN(strings.TrimSpace(line[1:]), " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid file info: %q", strings.TrimSpace(line))
	}
	fileSize, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing file size: %w", err)
	}

	if _, err := conn.Write([]byte{0}); err != nil {
		return nil, err
	}

	content := make([]byte, fileSize)
	received := 0
	for received < len(content) {
		n, err := r.Read(content[received:])
		received += n
		if err != nil {
			return nil, fmt.Errorf("reading file data: %w", err)
		}
	}

	if err := rcmd.ReadAck(r); err != nil {
		return nil, fmt.Errorf("transfer failed: %w", err)
	}
	if _, err := conn.Write([]byte{0}); err != nil {
		return nil, err
	}

	return content, nil
}

func WriteFile(hostname, username, remotePath, port string, content []byte) error {
	localUser, err := rcmd.LocalUser()
	if err != nil {
		return err
	}
	if username == "" {
		username = localUser
	}

	conn, err := rcmd.Dial(hostname, port)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = rcmd.SendRequest(conn, localUser, username, "rcp -t "+remotePath)
	if err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	if err := rcmd.ReadAck(r); err != nil {
		return fmt.Errorf("rsh %s: %w", hostname, err)
	}
	if err := rcmd.ReadAck(r); err != nil {
		return fmt.Errorf("rcp: %w", err)
	}

	fileInfo := fmt.Sprintf("C%04o %d %s\n", 0644, len(content), filepath.Base(remotePath))
	if _, err := conn.Write([]byte(fileInfo)); err != nil {
		return err
	}
	if err := rcmd.ReadAck(r); err != nil {
		return fmt.Errorf("file info rejected: %w", err)
	}

	if _, err := conn.Write(content); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{0}); err != nil {
		return err
	}
	if err := rcmd.ReadAck(r); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	if _, err := conn.Write([]byte("E\n")); err != nil {
		return err
	}
	if err := rcmd.ReadAck(r); err != nil {
		return fmt.Errorf("end of transfer rejected: %w", err)
	}

	return nil
}
