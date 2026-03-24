package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rrudol/frisco/internal/session"
)

// DataFormat matches Python request_json data_format.
type DataFormat string

const (
	FormatJSON DataFormat = "json"
	FormatForm DataFormat = "form"
	FormatRaw  DataFormat = "raw"
)

// RequestOpts bundles optional arguments for RequestJSON.
type RequestOpts struct {
	Query        []string
	Data         any
	DataFormat   DataFormat
	ExtraHeaders map[string]string
	Client       *http.Client
}

// MakeURL joins base with path or returns absolute URL.
func MakeURL(baseURL, pathOrURL string) (string, error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL, nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(pathOrURL)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func tokenString(s *session.Session) string {
	if s == nil || s.Token == nil {
		return ""
	}
	switch t := s.Token.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func headerKeyPresent(h map[string]string, key string) bool {
	for k := range h {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// RequestJSON performs HTTP request like Python request_json.
func RequestJSON(s *session.Session, method, pathOrURL string, opts RequestOpts) (any, error) {
	return requestJSONWithAutoRefresh(s, method, pathOrURL, opts, true)
}

func requestJSONWithAutoRefresh(
	s *session.Session,
	method, pathOrURL string,
	opts RequestOpts,
	allowAutoRefresh bool,
) (any, error) {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = session.DefaultBaseURL
	}
	fullURL, err := MakeURL(baseURL, pathOrURL)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	for _, p := range opts.Query {
		idx := strings.IndexByte(p, '=')
		if idx < 0 {
			return nil, fmt.Errorf("Zły parametr query: %s. Oczekiwane key=value", p)
		}
		params.Add(p[:idx], p[idx+1:])
	}
	if len(params) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for k, vs := range params {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	headers := make(map[string]string)
	for k, v := range s.Headers {
		headers[k] = v
	}
	if tok := tokenString(s); tok != "" && !headerKeyPresent(headers, "authorization") {
		headers["Authorization"] = "Bearer " + tok
	}
	for k, v := range opts.ExtraHeaders {
		headers[k] = v
	}

	var bodyReader io.Reader
	if opts.Data != nil {
		switch opts.DataFormat {
		case FormatJSON:
			b, err := json.Marshal(opts.Data)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(b)
			if !headerKeyPresent(headers, "content-type") {
				headers["Content-Type"] = "application/json"
			}
		case FormatForm:
			switch d := opts.Data.(type) {
			case map[string]any:
				uv := url.Values{}
				for k, v := range d {
					uv.Set(k, fmt.Sprint(v))
				}
				bodyReader = strings.NewReader(uv.Encode())
			case string:
				bodyReader = strings.NewReader(d)
			default:
				return nil, fmt.Errorf("Dla data_format=form podaj dict albo string.")
			}
			if !headerKeyPresent(headers, "content-type") {
				headers["Content-Type"] = "application/x-www-form-urlencoded"
			}
		case FormatRaw:
			str, ok := opts.Data.(string)
			if !ok {
				return nil, fmt.Errorf("Dla data_format=raw podaj string.")
			}
			bodyReader = strings.NewReader(str)
		default:
			return nil, fmt.Errorf("Nieobsługiwany data_format. Użyj: json, form, raw.")
		}
	}

	req, err := http.NewRequest(strings.ToUpper(method), fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := opts.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Błąd połączenia: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	text := string(raw)

	if resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusUnauthorized && allowAutoRefresh && !isTokenEndpoint(fullURL) {
			if refreshed, refreshErr := refreshAccessToken(s, opts.Client); refreshErr == nil && refreshed {
				return requestJSONWithAutoRefresh(s, method, pathOrURL, opts, false)
			}
		}
		msg := map[string]any{
			"status": resp.StatusCode,
			"reason": http.StatusText(resp.StatusCode),
			"url":    fullURL,
			"body":   text,
		}
		b, _ := json.MarshalIndent(msg, "", "  ")
		return nil, fmt.Errorf("%s", string(b))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		if len(text) == 0 {
			return map[string]any{}, nil
		}
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	return map[string]any{"status": resp.StatusCode, "body": text}, nil
}

func isTokenEndpoint(fullURL string) bool {
	return strings.Contains(fullURL, "/app/commerce/connect/token")
}

func refreshAccessToken(s *session.Session, client *http.Client) (bool, error) {
	rt := session.RefreshTokenString(s)
	if rt == "" {
		return false, fmt.Errorf("brak refresh tokena")
	}
	payload := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": rt,
	}
	result, err := requestJSONWithAutoRefresh(s, "POST", "/app/commerce/connect/token", RequestOpts{
		Data:       payload,
		DataFormat: FormatForm,
		Client:     client,
	}, false)
	if err != nil {
		return false, err
	}
	m, ok := result.(map[string]any)
	if !ok {
		return false, fmt.Errorf("nieoczekiwana odpowiedź od endpointu token")
	}
	accessToken, _ := m["access_token"].(string)
	if strings.TrimSpace(accessToken) == "" {
		return false, fmt.Errorf("brak access_token w odpowiedzi refresh")
	}
	s.Token = accessToken
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	s.Headers["Authorization"] = "Bearer " + accessToken
	if newRefresh, ok := m["refresh_token"].(string); ok && strings.TrimSpace(newRefresh) != "" {
		s.RefreshToken = newRefresh
	}
	if err := session.Save(s); err != nil {
		return false, err
	}
	return true, nil
}
