package rcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tenox7/rsh-mcp/internal/rcmd"
)

type Copied struct {
	Path string
	Size int64
}

type Report struct {
	Files  []Copied
	Errors []string
}

// Download copies remote files into localDir. With login (a remote
// without /bin/sh, such as OpenVMS) paths are sent as given and each one
// gets its own rcp session, since OpenVMS rcp takes one source per
// command; wildcards within a path still work.
func Download(ctx context.Context, hostname, username, port string, paths []string, localDir string, recursive, login bool) (*Report, error) {
	if !login || len(paths) == 1 {
		return download(ctx, hostname, username, port, paths, localDir, recursive, login)
	}
	all := &Report{}
	for _, p := range paths {
		rep, err := download(ctx, hostname, username, port, []string{p}, localDir, recursive, login)
		merge(all, rep, p, err)
		if ctx.Err() != nil {
			return all, errors.New("cancelled")
		}
	}
	return all, nil
}

func merge(all, rep *Report, path string, err error) {
	if rep != nil {
		all.Files = append(all.Files, rep.Files...)
		all.Errors = append(all.Errors, rep.Errors...)
	}
	if err != nil {
		all.Errors = append(all.Errors, path+": "+err.Error())
	}
}

func download(ctx context.Context, hostname, username, port string, paths []string, localDir string, recursive, login bool) (*Report, error) {
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, err
	}
	command := "rcp -f " + flag(recursive, "-r ") + quoteAll(paths, login)
	conn, r, err := open(hostname, username, port, command)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer context.AfterFunc(ctx, func() { conn.Close() })()

	rep := &Report{}
	dirs := []string{localDir}
	if err := ack(conn); err != nil {
		return rep, err
	}
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF && line == "" {
			return rep, nil
		}
		if err != nil {
			return rep, cancelled(ctx, err)
		}
		switch line[0] {
		case 1:
			rep.Errors = append(rep.Errors, strings.TrimSpace(line[1:]))
			continue
		case 2:
			return rep, errors.New(strings.TrimSpace(line[1:]))
		case 'T':
		case 'E':
			if len(dirs) > 1 {
				dirs = dirs[:len(dirs)-1]
			}
		case 'D':
			mode, _, name, err := parseControl(line)
			if err != nil {
				return rep, err
			}
			dir := filepath.Join(dirs[len(dirs)-1], name)
			if err := os.MkdirAll(dir, mode|0700); err != nil {
				return rep, err
			}
			dirs = append(dirs, dir)
		case 'C':
			mode, size, name, err := parseControl(line)
			if err != nil {
				return rep, err
			}
			path := filepath.Join(dirs[len(dirs)-1], name)
			if err := receive(conn, r, path, mode, size); err != nil {
				return rep, cancelled(ctx, err)
			}
			rep.Files = append(rep.Files, Copied{path, size})
			continue
		default:
			return rep, fmt.Errorf("unexpected rcp message: %q", strings.TrimSpace(line))
		}
		if err := ack(conn); err != nil {
			return rep, err
		}
	}
}

func receive(conn net.Conn, r *bufio.Reader, path string, mode os.FileMode, size int64) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode|0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := ack(conn); err != nil {
		return err
	}
	if _, err := io.CopyN(f, r, size); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := rcmd.ReadAck(r); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return ack(conn)
}

// Upload copies local files to remoteTarget. With login (a remote without
// /bin/sh, such as OpenVMS) the target is sent as given and each file gets
// its own rcp session with its full target name, since OpenVMS rcp rejects
// "rcp -t -d" as an ambiguous target. Folders can not be copied that way.
func Upload(ctx context.Context, hostname, username, port string, paths []string, remoteTarget string, recursive, login bool) (*Report, error) {
	if !login {
		return upload(ctx, hostname, username, port, paths, remoteTarget, recursive, false)
	}
	toDir := len(paths) > 1 || strings.ContainsAny(remoteTarget[len(remoteTarget)-1:], "/]>:")
	all := &Report{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if st.IsDir() {
			return nil, fmt.Errorf("%s is a folder: folders can not be copied with shell login", p)
		}
	}
	for _, p := range paths {
		target := remoteTarget
		if toDir {
			target = join(remoteTarget, filepath.Base(p))
		}
		rep, err := upload(ctx, hostname, username, port, []string{p}, target, false, true)
		merge(all, rep, p, err)
		if ctx.Err() != nil {
			return all, errors.New("cancelled")
		}
	}
	return all, nil
}

// join adds a file name to a remote folder: DKA0:[DIR] and DEV: take it as
// they are, a Unix-style folder after a slash.
func join(dir, name string) string {
	if strings.ContainsAny(dir[len(dir)-1:], "/]>:") {
		return dir + name
	}
	return dir + "/" + name
}

func upload(ctx context.Context, hostname, username, port string, paths []string, remoteTarget string, recursive, login bool) (*Report, error) {
	toDir := len(paths) > 1
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if st.IsDir() && !recursive {
			return nil, fmt.Errorf("%s is a folder: set recursive", p)
		}
		toDir = toDir || st.IsDir()
	}
	command := "rcp -t " + flag(toDir, "-d ") + flag(recursive, "-r ") + quote(remoteTarget, login)
	conn, r, err := open(hostname, username, port, command)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer context.AfterFunc(ctx, func() { conn.Close() })()

	if err := rcmd.ReadAck(r); err != nil {
		return nil, fmt.Errorf("rcp: %w", err)
	}
	rep := &Report{}
	for _, p := range paths {
		if err := send(conn, r, p, rep); err != nil {
			return rep, cancelled(ctx, err)
		}
	}
	return rep, nil
}

func send(conn net.Conn, r *bufio.Reader, path string, rep *Report) error {
	st, err := os.Stat(path)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return nil
	}
	if st.IsDir() {
		return sendDir(conn, r, path, st, rep)
	}
	f, err := os.Open(path)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return nil
	}
	defer f.Close()

	header := fmt.Sprintf("C%04o %d %s\n", st.Mode().Perm(), st.Size(), filepath.Base(path))
	if _, err := io.WriteString(conn, header); err != nil {
		return err
	}
	if err := rcmd.ReadAck(r); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return nil
	}
	if _, err := io.CopyN(conn, f, st.Size()); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := ack(conn); err != nil {
		return err
	}
	if err := rcmd.ReadAck(r); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return nil
	}
	rep.Files = append(rep.Files, Copied{path, st.Size()})
	return nil
}

func sendDir(conn net.Conn, r *bufio.Reader, path string, st os.FileInfo, rep *Report) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return nil
	}
	header := fmt.Sprintf("D%04o 0 %s\n", st.Mode().Perm(), filepath.Base(path))
	if _, err := io.WriteString(conn, header); err != nil {
		return err
	}
	if err := rcmd.ReadAck(r); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return nil
	}
	for _, e := range entries {
		if err := send(conn, r, filepath.Join(path, e.Name()), rep); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(conn, "E\n"); err != nil {
		return err
	}
	return rcmd.ReadAck(r)
}

func open(hostname, username, port, command string) (net.Conn, *bufio.Reader, error) {
	localUser, err := rcmd.LocalUser()
	if err != nil {
		return nil, nil, err
	}
	if username == "" {
		username = localUser
	}
	conn, err := rcmd.Dial(hostname, port)
	if err != nil {
		return nil, nil, err
	}
	if err := rcmd.SendRequest(conn, localUser, username, command); err != nil {
		conn.Close()
		return nil, nil, err
	}
	r := bufio.NewReader(conn)
	if err := rcmd.ReadAck(r); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("rsh %s: %w", hostname, err)
	}
	return conn, r, nil
}

func parseControl(line string) (os.FileMode, int64, string, error) {
	parts := strings.SplitN(strings.TrimSpace(line[1:]), " ", 3)
	if len(parts) != 3 {
		return 0, 0, "", fmt.Errorf("invalid rcp message: %q", strings.TrimSpace(line))
	}
	mode, err := strconv.ParseUint(parts[0], 8, 32)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid mode in %q", strings.TrimSpace(line))
	}
	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid size in %q", strings.TrimSpace(line))
	}
	name := parts[2]
	if name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		return 0, 0, "", fmt.Errorf("refusing remote file name %q", name)
	}
	return os.FileMode(mode).Perm(), size, name, nil
}

func ack(conn net.Conn) error {
	_, err := conn.Write([]byte{0})
	return err
}

func cancelled(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errors.New("cancelled")
	}
	return err
}

func flag(on bool, f string) string {
	if on {
		return f
	}
	return ""
}

var plainPath = regexp.MustCompile(`^[A-Za-z0-9_./*?~+,:=@%-]+$`)

func quote(path string, login bool) string {
	if login || plainPath.MatchString(path) {
		return path
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func quoteAll(paths []string, login bool) string {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = quote(p, login)
	}
	return strings.Join(quoted, " ")
}
