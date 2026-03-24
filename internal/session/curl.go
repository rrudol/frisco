package session

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/shlex"
)

// CurlData mirrors Python parse_curl result.
type CurlData struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    *string
}

var userPathRe = regexp.MustCompile(`/users/(\d+)`)

// ParseCurl parses a curl command line (Polish errors like Python).
func ParseCurl(curlCommand string) (*CurlData, error) {
	tokens, err := shlex.Split(curlCommand)
	if err != nil {
		return nil, fmt.Errorf("shlex: %w", err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("Pusty curl.")
	}
	if tokens[0] != "curl" {
		return nil, fmt.Errorf("Komenda musi zaczynać się od 'curl'.")
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
		return nil, fmt.Errorf("Nie udało się znaleźć URL w curl.")
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

// Allowed header keys for session from-curl (case preserved on store).
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

// ApplyFromCurl updates session from parsed curl (Python cmd_session_from_curl).
func ApplyFromCurl(s *Session, c *CurlData) {
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	for k, v := range c.Headers {
		if _, ok := fromCurlHeaderAllow[strings.ToLower(k)]; ok {
			s.Headers[k] = v
		}
	}
	if t := ExtractToken(c.Headers); t != "" {
		s.Token = t
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
		return ""
	}
	if v := vals.Get("refresh_token"); v != "" {
		return v
	}
	return ""
}

// ExtractRefreshTokenFromCookie reads rtokenN from Cookie header.
func ExtractRefreshTokenFromCookie(cookieHeader string) string {
	if cookieHeader == "" {
		return ""
	}
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(part[:idx])
		v := strings.TrimSpace(part[idx+1:])
		if k == "rtokenN" {
			if i := strings.IndexByte(v, '|'); i >= 0 {
				return v[i+1:]
			}
			return v
		}
	}
	return ""
}
