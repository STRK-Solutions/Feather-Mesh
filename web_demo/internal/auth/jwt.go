// Package auth validates real Cloudflare Access JWTs; there is no test-header
// authentication path. Synthetic signed tokens live only in tests.
package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Claims struct {
	Email string `json:"email"`
	Type  string `json:"type"`
	jwt.RegisteredClaims
}
type Validator struct {
	issuer               string
	client               *http.Client
	mu                   sync.Mutex
	keys                 map[string]*rsa.PublicKey
	expires, lastAttempt time.Time
	ttl, minRefresh      time.Duration
	clock                func() time.Time
}

func New(issuer string, client *http.Client) (*Validator, error) {
	u, e := url.Parse(issuer)
	if e != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".cloudflareaccess.com") || u.Hostname() == ".cloudflareaccess.com" || u.Port() != "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("exact HTTPS Cloudflare Access issuer required")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	copyClient := *client
	copyClient.Timeout = 5 * time.Second
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Validator{issuer: issuer, client: &copyClient, ttl: 15 * time.Minute, minRefresh: time.Minute, clock: time.Now}, nil
}
func (v *Validator) Validate(ctx context.Context, raw, audience string) (Claims, error) {
	var c Claims
	if len(raw) == 0 || len(raw) > 16384 || audience == "" {
		return c, errors.New("missing or excessive assertion")
	}
	token, e := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 {
			return nil, errors.New("invalid key ID")
		}
		if _, ok = t.Header["jku"]; ok {
			return nil, errors.New("untrusted key URL")
		}
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.issuer), jwt.WithAudience(audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(v.clock), jwt.WithStrictDecoding())
	if e != nil || !token.Valid {
		return Claims{}, errors.New("invalid Access assertion")
	}
	if len(c.Audience) != 1 || c.Audience[0] != audience || c.Subject == "" || c.Email == "" || c.IssuedAt == nil || c.ExpiresAt == nil || c.ExpiresAt.Time.Sub(c.IssuedAt.Time) > 8*time.Hour || c.Type != "app" {
		return Claims{}, errors.New("invalid Access identity claims")
	}
	return c, nil
}
func (v *Validator) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.clock()
	if key := v.keys[kid]; key != nil && now.Before(v.expires) {
		return key, nil
	}
	if now.Sub(v.lastAttempt) < v.minRefresh {
		return nil, errors.New("key refresh throttled")
	}
	v.lastAttempt = now
	req, e := http.NewRequestWithContext(ctx, "GET", v.issuer+"/cdn-cgi/access/certs", nil)
	if e != nil {
		return nil, e
	}
	resp, e := v.client.Do(req)
	if e != nil {
		return nil, errors.New("Access keys unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("Access keys unavailable")
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if e != nil || len(body) > 256<<10 {
		return nil, errors.New("invalid key response")
	}
	var doc struct {
		Certs []struct {
			Kid  string `json:"kid"`
			Cert string `json:"cert"`
		} `json:"public_certs"`
	}
	if e = json.Unmarshal(body, &doc); e != nil || len(doc.Certs) == 0 || len(doc.Certs) > 16 {
		return nil, errors.New("invalid key set")
	}
	keys := map[string]*rsa.PublicKey{}
	for _, c := range doc.Certs {
		block, rest := pem.Decode([]byte(c.Cert))
		if block == nil || len(rest) != 0 || c.Kid == "" || len(c.Kid) > 256 {
			return nil, errors.New("invalid public certificate")
		}
		cert, e := x509.ParseCertificate(block.Bytes)
		if e != nil {
			return nil, errors.New("invalid public certificate")
		}
		key, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok || key.N.BitLen() < 2048 || keys[c.Kid] != nil {
			return nil, errors.New("invalid RSA key set")
		}
		keys[c.Kid] = key
	}
	v.keys = keys
	v.expires = now.Add(v.ttl)
	if key := keys[kid]; key != nil {
		return key, nil
	}
	return nil, errors.New("unknown signing key")
}
