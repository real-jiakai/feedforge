// Package fetch retrieves source pages safely: it blocks requests to
// private/internal networks (SSRF), bounds response size and time, and
// decodes legacy charsets to UTF-8.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

const userAgent = "FeedForge/1.0 (+https://github.com/real-jiakai/feedforge)"

// Fetcher is a hardened HTTP client for untrusted, user-supplied URLs.
type Fetcher struct {
	client   *http.Client
	maxBytes int64
}

// New builds a Fetcher. maxBytes bounds the decoded page size. When
// allowPrivate is false, connections to loopback, RFC1918, link-local and
// other non-public addresses are refused — the check runs at dial time on
// the resolved IP, so it also covers redirects and DNS tricks.
func New(allowPrivate bool, maxBytes int64, timeout time.Duration) *Fetcher {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if !allowPrivate {
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("connections to %s are not allowed", host)
			}
			return nil
		}
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          20,
		IdleConnTimeout:       60 * time.Second,
	}
	// A proxy would defeat the dial-time guard entirely: the only address
	// dialed is the proxy's, and the real target is passed along inside the
	// request for the proxy to resolve and fetch. Honour proxy environment
	// variables only when private addresses are explicitly permitted.
	if allowPrivate {
		transport.Proxy = http.ProxyFromEnvironment
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
	return &Fetcher{client: client, maxBytes: maxBytes}
}

// blockedPrefixes lists address ranges that must never be dialed on behalf
// of a user-supplied URL. net.IP.IsPrivate covers only RFC1918 + fc00::/7,
// which misses shared/CGNAT space (100.64.0.0/10 — the range Tailscale hands
// out), IETF protocol assignments, benchmarking and reserved space, and the
// IPv6 forms that embed an IPv4 address.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),          // "this network"
	netip.MustParsePrefix("10.0.0.0/8"),         // RFC1918
	netip.MustParsePrefix("100.64.0.0/10"),      // RFC6598 shared / CGNAT / Tailscale
	netip.MustParsePrefix("127.0.0.0/8"),        // loopback
	netip.MustParsePrefix("169.254.0.0/16"),     // link-local incl. cloud metadata
	netip.MustParsePrefix("172.16.0.0/12"),      // RFC1918
	netip.MustParsePrefix("192.0.0.0/24"),       // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),       // TEST-NET-1
	netip.MustParsePrefix("192.88.99.0/24"),     // 6to4 relay anycast
	netip.MustParsePrefix("192.168.0.0/16"),     // RFC1918
	netip.MustParsePrefix("198.18.0.0/15"),      // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"),    // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),     // TEST-NET-3
	netip.MustParsePrefix("224.0.0.0/4"),        // multicast
	netip.MustParsePrefix("240.0.0.0/4"),        // reserved
	netip.MustParsePrefix("255.255.255.255/32"), // broadcast
	netip.MustParsePrefix("::/128"),             // unspecified
	netip.MustParsePrefix("::1/128"),            // loopback
	netip.MustParsePrefix("::/96"),              // IPv4-compatible (deprecated)
	netip.MustParsePrefix("64:ff9b::/96"),       // NAT64 — embeds an IPv4 target
	netip.MustParsePrefix("64:ff9b:1::/48"),     // local-use NAT64
	netip.MustParsePrefix("100::/64"),           // discard-only
	netip.MustParsePrefix("2001:db8::/32"),      // documentation
	netip.MustParsePrefix("2002::/16"),          // 6to4 — embeds an IPv4 target
	netip.MustParsePrefix("fc00::/7"),           // unique local
	netip.MustParsePrefix("fe80::/10"),          // link-local
	netip.MustParsePrefix("ff00::/8"),           // multicast
}

func isPublicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	// Unmap so an IPv4-mapped IPv6 address (::ffff:127.0.0.1) is tested
	// against the IPv4 rules rather than sliding past them.
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// Result is a fetched, UTF-8-decoded page.
type Result struct {
	Content  string // decoded page text, newlines normalized to LF by callers
	FinalURL string // URL after redirects; base for resolving relative links
	Status   int
}

// Get fetches rawURL. encodingOverride, when non-empty, names a charset
// (e.g. "gbk") used instead of auto-detection.
func (f *Fetcher) Get(ctx context.Context, rawURL, encodingOverride string) (*Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q (use http or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("URL has no host")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if !allowedContentType(ctype) {
		return nil, fmt.Errorf("unsupported content type %q", ctype)
	}

	limited := io.LimitReader(resp.Body, f.maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if int64(len(raw)) > f.maxBytes {
		return nil, fmt.Errorf("page exceeds size limit (%d MB)", f.maxBytes/(1024*1024))
	}
	if looksBinary(raw) {
		return nil, errors.New("page looks like binary data, not text")
	}

	decoded, err := decode(raw, ctype, encodingOverride)
	if err != nil {
		return nil, err
	}

	finalURL := u.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return &Result{Content: decoded, FinalURL: finalURL, Status: resp.StatusCode}, nil
}

func allowedContentType(ctype string) bool {
	if ctype == "" {
		return true // sniffed by looksBinary instead
	}
	mt := strings.ToLower(strings.TrimSpace(strings.SplitN(ctype, ";", 2)[0]))
	if strings.HasPrefix(mt, "text/") {
		return true
	}
	switch mt {
	case "application/xhtml+xml", "application/xml", "application/json",
		"application/rss+xml", "application/atom+xml",
		"application/javascript", "application/x-javascript":
		return true
	}
	return false
}

func looksBinary(b []byte) bool {
	n := len(b)
	if n > 512 {
		n = 512
	}
	for _, c := range b[:n] {
		if c == 0 {
			return true
		}
	}
	return false
}

func decode(raw []byte, ctype, encodingOverride string) (string, error) {
	if encodingOverride != "" {
		enc, err := htmlindex.Get(encodingOverride)
		if err != nil {
			return "", fmt.Errorf("unknown encoding %q", encodingOverride)
		}
		out, _, err := transform.Bytes(enc.NewDecoder(), raw)
		if err != nil {
			return "", fmt.Errorf("decoding as %s: %w", encodingOverride, err)
		}
		return string(out), nil
	}
	r, err := charset.NewReader(strings.NewReader(string(raw)), ctype)
	if err != nil {
		return "", fmt.Errorf("charset detection: %w", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("decoding page: %w", err)
	}
	return string(out), nil
}
