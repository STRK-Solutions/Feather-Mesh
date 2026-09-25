package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func cert(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	b, e := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test only"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}, &x509.Certificate{SerialNumber: big.NewInt(1)}, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: b}))
}
func TestJWTSignatureAudienceRotationAndBoundedOffline(t *testing.T) {
	ctx := context.Background()
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	next, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Now().Truncate(time.Second)
	kid := "old"
	public := cert(t, key)
	calls := 0
	offline := false
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://demo.cloudflareaccess.com/cdn-cgi/access/certs" {
			t.Fatal("caller controlled endpoint")
		}
		status := 200
		if offline {
			status = 503
		}
		b, _ := json.Marshal(map[string]any{"public_certs": []any{map[string]string{"kid": kid, "cert": public}}})
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})}
	v, e := New("https://demo.cloudflareaccess.com", client)
	if e != nil {
		t.Fatal(e)
	}
	v.clock = func() time.Time { return now }
	claims := Claims{Email: "user@example.invalid", Type: "app", RegisteredClaims: jwt.RegisteredClaims{Subject: "subject", Issuer: v.issuer, Audience: jwt.ClaimStrings{"portal"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}
	sign := func(k *rsa.PrivateKey, id string, c Claims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
		token.Header["kid"] = id
		s, e := token.SignedString(k)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	raw := sign(key, "old", claims)
	if _, e = v.Validate(ctx, raw, "portal"); e != nil {
		t.Fatal(e)
	}
	for name, c := range map[string]Claims{"wrong audience": func() Claims { x := claims; x.Audience = jwt.ClaimStrings{"admin"}; return x }(), "extra audience": func() Claims { x := claims; x.Audience = jwt.ClaimStrings{"portal", "admin"}; return x }(), "wrong issuer": func() Claims { x := claims; x.Issuer = "https://evil.invalid"; return x }(), "expired": func() Claims { x := claims; x.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Second)); return x }(), "missing expiry": func() Claims { x := claims; x.ExpiresAt = nil; return x }(), "missing email": func() Claims { x := claims; x.Email = ""; return x }()} {
		t.Run(name, func(t *testing.T) {
			if _, e := v.Validate(ctx, sign(key, "old", c), "portal"); e == nil {
				t.Fatal("invalid token admitted")
			}
		})
	}
	if _, e = v.Validate(ctx, sign(next, "old", claims), "portal"); e == nil {
		t.Fatal("forged signature")
	}
	if _, e = v.Validate(ctx, "malformed", "portal"); e == nil {
		t.Fatal("malformed accepted")
	}
	offline = true
	if _, e = v.Validate(ctx, raw, "portal"); e != nil {
		t.Fatal("bounded cached key unavailable")
	}
	now = now.Add(16 * time.Minute)
	if _, e = v.Validate(ctx, raw, "portal"); e == nil {
		t.Fatal("expired offline key trusted")
	}
	offline = false
	kid = "new"
	public = cert(t, next)
	now = now.Add(time.Minute)
	if _, e = v.Validate(ctx, sign(next, "new", claims), "portal"); e != nil {
		t.Fatal("rotation failed", e)
	}
	before := calls
	for i := 0; i < 20; i++ {
		v.Validate(ctx, sign(next, "unknown", claims), "portal")
	}
	if calls != before {
		t.Fatal("unknown kid bypassed refresh bound")
	}
	if _, e = v.Validate(ctx, raw, "portal"); e == nil {
		t.Fatal("removed key remained cached")
	}
}
func TestIssuerConfiguration(t *testing.T) {
	for _, issuer := range []string{"http://x.cloudflareaccess.com", "https://x.cloudflareaccess.com.evil.invalid", "https://x.cloudflareaccess.com/path", "https://x.cloudflareaccess.com:443"} {
		if _, e := New(issuer, nil); e == nil {
			t.Fatal("unsafe issuer", issuer)
		}
	}
}

type closer struct{ closed bool }

func (c *closer) Close() error { c.closed = true; return nil }
func TestStreamsRevokeAndExpiry(t *testing.T) {
	var s Streams
	c := &closer{}
	s.Track("a", c, time.Now().Add(time.Hour), func(context.Context) bool { return true })
	s.Revoke("a")
	if !c.closed {
		t.Fatal("stream survived revoke")
	}
	c = &closer{}
	s.Track("b", c, time.Now().Add(-time.Second), func(context.Context) bool { return true })
	s.Check(context.Background())
	if !c.closed {
		t.Fatal("stream survived token expiry")
	}
	c = &closer{}
	s.Track("c", c, time.Now().Add(time.Hour), func(context.Context) bool { return false })
	s.Check(context.Background())
	if !c.closed {
		t.Fatal("stream survived local policy change")
	}
}
