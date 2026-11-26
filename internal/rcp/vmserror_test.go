package rcp

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
)

const vmsError = "%RCP-E-OPENIN, error opening SYS$SYSDEVICE:[U]NONE.TXT; as input"

// vmsSource answers "rcp -f" the way OpenVMS TCPIP$RCP does for a
// missing file: a message line with no status byte.
func vmsSource(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
	r.ReadByte()
	io.WriteString(conn, vmsError+"\r\n")
}

func TestDownloadVMSError(t *testing.T) {
	port, _ := fakeRshd(t, vmsSource)
	rep, err := Download(context.Background(), "127.0.0.1", "u", port, []string{"NONE.TXT"}, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 0 || len(rep.Errors) != 1 || rep.Errors[0] != vmsError {
		t.Errorf("report %+v, want the error %q", rep, vmsError)
	}
}

func TestReadFileVMSError(t *testing.T) {
	port, _ := fakeRshd(t, vmsSource)
	_, err := ReadFile("127.0.0.1", "u", "NONE.TXT", port)
	if err == nil || err.Error() != vmsError {
		t.Errorf("error %v, want %q", err, vmsError)
	}
}

func TestWriteFileVMSError(t *testing.T) {
	port, _ := fakeRshd(t, func(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
		io.WriteString(conn, "%RCP-E-OPENOUT, error opening [NODIR]A.TXT as output\n")
	})
	err := WriteFile("127.0.0.1", "u", "[NODIR]A.TXT", port, []byte("a"))
	if err == nil || !strings.Contains(err.Error(), "%RCP-E-OPENOUT") {
		t.Errorf("error %v, want the OpenVMS message", err)
	}
}

func TestUploadVMSError(t *testing.T) {
	p := t.TempDir() + "/a.txt"
	if err := os.WriteFile(p, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	port, _ := fakeRshd(t, func(_ rcmdRequest, r *bufio.Reader, conn net.Conn) {
		io.WriteString(conn, "%RCP-E-OPENOUT, error opening [NODIR] as output\n")
	})
	_, err := Upload(context.Background(), "127.0.0.1", "u", port, []string{p}, "[NODIR]", false)
	if err == nil || !strings.Contains(err.Error(), "%RCP-E-OPENOUT") {
		t.Errorf("error %v, want the OpenVMS message", err)
	}
}
