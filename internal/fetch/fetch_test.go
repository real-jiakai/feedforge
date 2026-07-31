package fetch

import (
	"net"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip     string
		public bool
		why    string
	}{
		// Genuinely routable — these must keep working.
		{"1.1.1.1", true, "public IPv4"},
		{"93.184.216.34", true, "public IPv4"},
		{"2606:4700:4700::1111", true, "public IPv6"},

		// Classic private space.
		{"127.0.0.1", false, "loopback"},
		{"10.1.2.3", false, "RFC1918"},
		{"172.16.0.1", false, "RFC1918"},
		{"192.168.1.1", false, "RFC1918"},
		{"0.0.0.0", false, "unspecified"},
		{"255.255.255.255", false, "broadcast"},
		{"224.0.0.1", false, "multicast"},

		// Cloud metadata — the highest-value SSRF target.
		{"169.254.169.254", false, "link-local / cloud metadata"},

		// Ranges net.IP.IsPrivate does not cover.
		{"100.64.0.1", false, "CGNAT / Tailscale"},
		{"100.100.100.100", false, "CGNAT / Tailscale"},
		{"192.0.0.1", false, "IETF protocol assignments"},
		{"198.18.0.1", false, "benchmarking"},
		{"240.0.0.1", false, "reserved"},

		// IPv6 forms, including those embedding an IPv4 address.
		{"::1", false, "IPv6 loopback"},
		{"::", false, "IPv6 unspecified"},
		{"fc00::1", false, "unique local"},
		{"fd12:3456::1", false, "unique local"},
		{"fe80::1", false, "link-local"},
		{"ff02::1", false, "multicast"},
		{"::ffff:127.0.0.1", false, "IPv4-mapped loopback"},
		{"::ffff:10.0.0.1", false, "IPv4-mapped RFC1918"},
		{"::127.0.0.1", false, "IPv4-compatible loopback"},
		{"64:ff9b::7f00:1", false, "NAT64-embedded loopback"},
		{"2002:7f00:1::", false, "6to4-embedded loopback"},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := isPublicIP(ip); got != c.public {
			t.Errorf("isPublicIP(%s) = %v, want %v (%s)", c.ip, got, c.public, c.why)
		}
	}
}

func TestAllowedContentType(t *testing.T) {
	ok := []string{
		"", "text/html", "text/html; charset=utf-8", "TEXT/PLAIN",
		"application/xhtml+xml", "application/json", "application/rss+xml",
	}
	for _, c := range ok {
		if !allowedContentType(c) {
			t.Errorf("allowedContentType(%q) = false, want true", c)
		}
	}
	bad := []string{"image/png", "application/pdf", "video/mp4", "application/octet-stream"}
	for _, c := range bad {
		if allowedContentType(c) {
			t.Errorf("allowedContentType(%q) = true, want false", c)
		}
	}
}

func TestLooksBinary(t *testing.T) {
	if !looksBinary([]byte{0x89, 'P', 'N', 'G', 0x00, 0x1a}) {
		t.Error("PNG header should look binary")
	}
	if looksBinary([]byte("<html><body>hi</body></html>")) {
		t.Error("HTML should not look binary")
	}
}
