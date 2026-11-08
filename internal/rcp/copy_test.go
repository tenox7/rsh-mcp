package rcp

import "testing"

func TestModeFlagComesLastForOldRcpThatStartsOnIt(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{sourceCommand([]string{"/etc/hosts"}, false), "rcp -f /etc/hosts"},
		{sourceCommand([]string{"/usr/a", "/usr/b"}, true), "rcp -r -f /usr/a /usr/b"},
		{sinkCommand("/tmp", false, false), "rcp -t /tmp"},
		{sinkCommand("/tmp", true, false), "rcp -d -t /tmp"},
		{sinkCommand("/tmp", true, true), "rcp -r -d -t /tmp"},
		{sinkCommand("/tmp/my dir", true, false), "rcp -d -t '/tmp/my dir'"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
