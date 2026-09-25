package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

// PublicAddress excludes LAN, metadata, reserved, multicast and mapped IPv6.
// DNS answers are checked immediately before dialing the selected literal IP;
// an HTTP proxy or a second resolver cannot bypass that decision.
func PublicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

func sourceClient(source Source, limits Limits) *http.Client {
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxResponseHeaderBytes: 32 * 1024,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" {
				return nil, errors.New("source port rejected")
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, errors.New("source DNS unavailable")
			}
			for _, ip := range ips {
				if !PublicAddress(ip) {
					return nil, errors.New("source DNS includes forbidden address")
				}
			}
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}
	return &http.Client{Transport: transport, Timeout: time.Duration(limits.ElapsedSeconds) * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > limits.Redirects {
			return errors.New("redirect limit")
		}
		return validateURL(source.Importer, req.URL.String())
	}}
}

// Fetch downloads only an already approved, checksum-pinned source. No retries.
// Archives are intentionally unsupported: expanded bytes and entries are zero.
func Fetch(ctx context.Context, source Source, limits Limits, destination string) error {
	if source.MaxBytes < 1 || source.MaxBytes > limits.SourceBytes {
		return errors.New("source byte bound missing")
	}
	limits.SourceBytes = source.MaxBytes
	if err := validateURL(source.Importer, source.URL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")
	response, err := sourceClient(source, limits).Do(req)
	if err != nil {
		return errors.New("source fetch failed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.ContentLength > limits.SourceBytes || response.Header.Get("Content-Encoding") != "" {
		return errors.New("source response rejected")
	}
	return writePinned(response.Body, destination, source.SHA256, limits.SourceBytes)
}

func writePinned(input io.Reader, destination, expected string, limit int64) error {
	if !digestPattern.MatchString(expected) || limit < 1 {
		return errors.New("missing source bound or checksum")
	}
	f, err := os.CreateTemp(filepath.Dir(destination), ".download-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(input, limit+1))
	if err != nil || n > limit || hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("partial, oversized or checksum-mismatched source")
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Never overwrite a pinned artifact, including an attacker-controlled link.
	if err = os.Link(f.Name(), destination); err != nil {
		return err
	}
	return syncDir(filepath.Dir(destination))
}
