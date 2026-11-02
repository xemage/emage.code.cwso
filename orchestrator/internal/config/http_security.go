package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// MinJWTSecretBytes is the minimum length of the HS256 signing key
// (SEC-T695-07). RFC 7518 section 3.2 requires an HMAC key at least as long
// as the hash output, 256 bits for HS256. `openssl rand -hex 32` (what
// scripts/cwso-bootstrap-secrets.sh runs) yields 64 characters.
const MinJWTSecretBytes = 32

// DefaultHTTPAddr is the listen address when neither the -addr flag nor
// CWSO_HTTP_ADDR is set (SEC-T695-10): loopback only. Listening on another
// interface is an explicit operator choice, e.g. CWSO_HTTP_ADDR=":8080" inside
// a container, where the host-side port publish decides the exposure.
const DefaultHTTPAddr = "127.0.0.1:8080"

// ValidateHTTPSecurity checks the settings that guard the HTTP transport and
// normalizes the Origin allow-list in place. Load calls it; the command
// calls it again after its flags override Transport or HTTPAddr, so a
// flag-selected HTTP transport gets the same checks as an env-selected one.
func (c *Config) ValidateHTTPSecurity() error {
	if err := validateJWTSecret(c.Transport, c.JWTSecret); err != nil {
		return err
	}
	origins, err := NormalizeOrigins(c.AllowedOrigins)
	if err != nil {
		return err
	}
	c.AllowedOrigins = origins
	return nil
}

// validateJWTSecret enforces presence and minimum length of the HS256 key
// when the HTTP transport is selected. The error never contains the secret
// or its length.
func validateJWTSecret(transport, secret string) error {
	if transport != "http" {
		return nil
	}
	if secret == "" {
		// Immutable constraint: no bypass of JWT auth, even in dev.
		return fmt.Errorf("JWT secret must be set via /run/secrets/jwt_secret or CWSO_JWT_SECRET when transport=http")
	}
	if len(secret) < MinJWTSecretBytes {
		return fmt.Errorf("JWT secret is shorter than %d bytes; generate one with `openssl rand -hex 32` (scripts/cwso-bootstrap-secrets.sh)", MinJWTSecretBytes)
	}
	return nil
}

// NormalizeOrigins validates every CWSO_ALLOWED_ORIGINS entry (SEC-T695-03)
// and returns the lower-cased, de-duplicated list in input order. An empty
// list, `*`, `null`, an entry without an http(s) scheme or a host, and an
// entry with userinfo, a path, a query or a fragment are rejected.
func NormalizeOrigins(entries []string) ([]string, error) {
	if len(entries) == 0 {
		return nil, errors.New("CWSO_ALLOWED_ORIGINS has no entries; list at least one origin such as http://127.0.0.1")
	}
	out := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		origin, err := normalizeOrigin(entry)
		if err != nil {
			return nil, fmt.Errorf("CWSO_ALLOWED_ORIGINS entry %q: %w", entry, err)
		}
		if _, dup := seen[origin]; dup {
			continue
		}
		seen[origin] = struct{}{}
		out = append(out, origin)
	}
	return out, nil
}

// normalizeOrigin turns one entry into the exact `scheme://host[:port]` form
// that browsers send in the Origin header, or explains why it cannot match.
func normalizeOrigin(entry string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(entry))
	switch e {
	case "*":
		return "", errors.New("a wildcard is not allowed; list each origin")
	case "null":
		return "", errors.New(`"null" is not allowed: sandboxed frames and file: pages send it, so it admits any of them`)
	}
	if strings.ContainsAny(e, "?#") {
		return "", errors.New("a query or fragment is not allowed; use scheme://host[:port]")
	}
	u, err := url.Parse(e)
	if err != nil {
		return "", errors.New("not a valid URL; use scheme://host[:port]")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("the scheme must be http or https; use scheme://host[:port]")
	}
	if err := checkOriginParts(u); err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

// checkOriginParts rejects the URL parts an Origin header never carries.
func checkOriginParts(u *url.URL) error {
	if u.Opaque != "" || u.Hostname() == "" {
		return errors.New("a host is required; use scheme://host[:port]")
	}
	if u.User != nil {
		return errors.New("userinfo is not allowed; use scheme://host[:port]")
	}
	if u.Path != "" || u.RawPath != "" {
		return errors.New("a path (even a trailing /) is not allowed; use scheme://host[:port]")
	}
	if strings.HasSuffix(u.Host, ":") {
		return errors.New("an empty port is not allowed; use scheme://host[:port]")
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("the port must be between 1 and 65535")
		}
	}
	return nil
}
