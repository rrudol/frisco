package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/shlex"
)

// CurlData holds the parsed components of a curl command.
type CurlData struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    *string
}

// userPathRe matches the numeric user ID segment in a Frisco API path.
var userPathRe = regexp.MustCompile(`/users/(\d+)`)

// ParseCurl parses a curl command string into its URL, method, headers, and body.
func ParseCurl(curlCommand string) (*CurlData, error) {
	tokens, err := shlex.Split(curlCommand)
	if err != nil {
		return nil, fmt.Errorf("shlex: %w", err)
	}
	if len(tokens) == 0 {
		return nil, errors.New("Empty curl command.")
	}
	if tokens[0] != "curl" {
		return nil, errors.New("Command must start with 'curl'.")
	}

	method := "GET"
	rawURL := ""
	headers := map[string]string{}
	var body *string

	i := 1
	for i < len(tokens) {
		token := tokens[i]
		var nxt string
		if i+1 < len(tokens) {
			nxt = tokens[i+1]
		}

		switch {
		case (token == "-X" || token == "--request") && nxt != "":
			method = strings.ToUpper(nxt)
			i += 2
			continue
		case (token == "-H" || token == "--header") && nxt != "":
			if idx := strings.IndexByte(nxt, ':'); idx >= 0 {
				k := strings.TrimSpace(nxt[:idx])
				v := strings.TrimSpace(nxt[idx+1:])
				headers[k] = v
			}
			i += 2
			continue
		case (token == "--data" || token == "--data-raw" || token == "--data-binary" || token == "-d") && nxt != "":
			body = &nxt
			if method == "GET" {
				method = "POST"
			}
			i += 2
			continue
		case token == "--url" && nxt != "":
			rawURL = nxt
			i += 2
			continue
		case strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://"):
			rawURL = token
			i++
			continue
		}
		i++
	}

	if rawURL == "" {
		return nil, errors.New("Could not find URL in curl.")
	}

	return &CurlData{Method: method, URL: rawURL, Headers: headers, Body: body}, nil
}

// ExtractToken from Authorization: Bearer ...
func ExtractToken(headers map[string]string) string {
	for k, v := range headers {
		if !strings.EqualFold(k, "authorization") {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(v), " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// ExtractUserID from URL path /users/{id}/...
func ExtractUserID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	m := userPathRe.FindStringSubmatch(u.Path)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// fromCurlHeaderAllow is the set of header names (lowercase) that are imported
// into the session when parsing a curl command.
var fromCurlHeaderAllow = map[string]struct{}{
	"authorization":    {},
	"content-type":     {},
	"cookie":           {},
	"x-api-version":    {},
	"x-requested-with": {},
	"accept":           {},
	"origin":           {},
	"referer":          {},
}

// ApplyFromCurl updates a session with data extracted from a parsed curl command.
func ApplyFromCurl(s *Session, c *CurlData) {
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	for k, v := range c.Headers {
		if _, ok := fromCurlHeaderAllow[strings.ToLower(k)]; ok {
			s.Headers[k] = v
		}
	}
	s.Headers = NormalizeHeaders(s.Headers)
	if t := ExtractToken(c.Headers); t != "" {
		s.Token = t
		s.Headers["Authorization"] = "Bearer " + t
	}
	if rt := ExtractRefreshTokenFromCurlBody(c.Body); rt != "" {
		s.RefreshToken = rt
	}
	cookie := c.Headers["cookie"]
	if cookie == "" {
		cookie = c.Headers["Cookie"]
	}
	if rt := ExtractRefreshTokenFromCookie(cookie); rt != "" {
		s.RefreshToken = rt
	}
	if uid := ExtractUserID(c.URL); uid != "" {
		s.UserID = uid
	}
	if u, err := url.Parse(c.URL); err == nil && u.Scheme != "" && u.Host != "" {
		s.BaseURL = u.Scheme + "://" + u.Host
	}
}

// ExtractRefreshTokenFromCurlBody parses application/x-www-form-urlencoded body.
func ExtractRefreshTokenFromCurlBody(body *string) string {
	if body == nil {
		return ""
	}
	vals, err := url.ParseQuery(*body)
	if err != nil {
		// Continue with JSON fallback.
	} else if v := vals.Get("refresh_token"); v != "" {
		return v
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(*body), &obj); err == nil {
		if v, ok := obj["refresh_token"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extractRefreshTokenValue URL-decodes raw and strips a leading pipe-delimited
// prefix (Frisco encodes the token as "userId|token").
func extractRefreshTokenValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if dec, err := url.QueryUnescape(raw); err == nil && dec != "" {
		raw = dec
	}
	if i := strings.IndexByte(raw, '|'); i >= 0 {
		return strings.TrimSpace(raw[i+1:])
	}
	return raw
}

// ExtractRefreshTokenFromCookie reads token from cookie-like header values.
func ExtractRefreshTokenFromCookie(cookieHeader string) string {
	if cookieHeader == "" {
		return ""
	}
	// Handle both "Cookie: a=b; rtokenN=..." and Set-Cookie-like chunks.
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(part[:idx])
		v := strings.TrimSpace(part[idx+1:])
		lk := strings.ToLower(k)
		if strings.Contains(lk, "rtoken") {
			if token := extractRefreshTokenValue(v); token != "" {
				return token
			}
		}
	}

	// Fallback: Set-Cookie can appear as multiline/combined string.
	re := regexp.MustCompile(`(?i)([a-z0-9_-]*rtoken[a-z0-9_-]*)=([^;\s,]+)`)
	if m := re.FindStringSubmatch(cookieHeader); len(m) > 2 {
		return extractRefreshTokenValue(m[2])
	}
	return ""
}

// ExtractRefreshTokenFromHeaderValue extracts a refresh token from a raw
// Cookie or Set-Cookie header value string.
func ExtractRefreshTokenFromHeaderValue(value string) string {
	if value == "" {
		return ""
	}
	if t := ExtractRefreshTokenFromCookie(value); t != "" {
		return t
	}
	return ""
}
