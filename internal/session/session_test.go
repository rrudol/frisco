package session

import (
	"strings"
	"testing"
)

// ============================================================================
// ParseCurl
// ============================================================================

func TestParseCurl_BasicGET(t *testing.T) {
	c, err := ParseCurl(`curl https://www.frisco.pl/api/v1/products`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Method != "GET" {
		t.Errorf("method: got %q, want GET", c.Method)
	}
	if c.URL != "https://www.frisco.pl/api/v1/products" {
		t.Errorf("url: got %q", c.URL)
	}
	if c.Body != nil {
		t.Error("body should be nil")
	}
}

func TestParseCurl_WithMethod(t *testing.T) {
	c, err := ParseCurl(`curl -X DELETE https://www.frisco.pl/api/v1/cart/item/42`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Method != "DELETE" {
		t.Errorf("method: got %q, want DELETE", c.Method)
	}
}

func TestParseCurl_WithLongMethod(t *testing.T) {
	c, err := ParseCurl(`curl --request POST https://www.frisco.pl/api/v1/cart`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Method != "POST" {
		t.Errorf("method: got %q, want POST", c.Method)
	}
}

func TestParseCurl_HeadersParsed(t *testing.T) {
	cmd := `curl -H "Authorization: Bearer tok123" -H "Accept: application/json" https://www.frisco.pl/api/v1/products`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Headers["Authorization"] != "Bearer tok123" {
		t.Errorf("Authorization header: got %q", c.Headers["Authorization"])
	}
	if c.Headers["Accept"] != "application/json" {
		t.Errorf("Accept header: got %q", c.Headers["Accept"])
	}
}

func TestParseCurl_LongHeaderFlag(t *testing.T) {
	cmd := `curl --header "Cookie: session=abc" https://www.frisco.pl/api/v1/products`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Headers["Cookie"] != "session=abc" {
		t.Errorf("Cookie header: got %q", c.Headers["Cookie"])
	}
}

func TestParseCurl_WithDataRaw(t *testing.T) {
	cmd := `curl --data-raw "grant_type=refresh_token&refresh_token=myrefresh" https://www.frisco.pl/api/auth/token`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Method != "POST" {
		t.Errorf("method should be POST when body present, got %q", c.Method)
	}
	if c.Body == nil || *c.Body != "grant_type=refresh_token&refresh_token=myrefresh" {
		t.Errorf("body: got %v", c.Body)
	}
}

func TestParseCurl_WithDataBinary(t *testing.T) {
	cmd := `curl --data-binary '{"key":"value"}' https://www.frisco.pl/api/v1/cart`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Method != "POST" {
		t.Errorf("method should be POST, got %q", c.Method)
	}
}

func TestParseCurl_WithShortDataFlag(t *testing.T) {
	cmd := `curl -d "foo=bar" https://www.frisco.pl/api/v1/cart`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Body == nil || *c.Body != "foo=bar" {
		t.Errorf("body: got %v", c.Body)
	}
}

func TestParseCurl_ExplicitMethodNotOverriddenByBody(t *testing.T) {
	// If -X PUT is given alongside body, method stays PUT (not overridden to POST).
	cmd := `curl -X PUT -d "foo=bar" https://www.frisco.pl/api/v1/cart`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Method != "PUT" {
		t.Errorf("method: got %q, want PUT", c.Method)
	}
}

func TestParseCurl_URLFlag(t *testing.T) {
	cmd := `curl --url https://www.frisco.pl/api/v1/products`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.URL != "https://www.frisco.pl/api/v1/products" {
		t.Errorf("url: got %q", c.URL)
	}
}

func TestParseCurl_EmptyCommand(t *testing.T) {
	_, err := ParseCurl("")
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestParseCurl_NotCurl(t *testing.T) {
	_, err := ParseCurl("wget https://example.com")
	if err == nil {
		t.Error("expected error when command doesn't start with curl")
	}
}

func TestParseCurl_MissingURL(t *testing.T) {
	_, err := ParseCurl(`curl -H "Authorization: Bearer tok" -X GET`)
	if err == nil {
		t.Error("expected error when no URL found")
	}
}

func TestParseCurl_UnquotedComplexCommand(t *testing.T) {
	// Realistic browser-copied curl with multiple headers.
	cmd := `curl 'https://www.frisco.pl/api/v1/users/12345/cart' ` +
		`-H 'Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.tok' ` +
		`-H 'x-api-version: 3' ` +
		`-H 'Cookie: rtokenN=12345%7Cmyrefreshtoken'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.URL != "https://www.frisco.pl/api/v1/users/12345/cart" {
		t.Errorf("url: got %q", c.URL)
	}
	if !strings.HasPrefix(c.Headers["Authorization"], "Bearer ") {
		t.Errorf("Authorization header not set correctly: %q", c.Headers["Authorization"])
	}
}

// ============================================================================
// ExtractToken
// ============================================================================

func TestExtractToken_Bearer(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer mytoken123"}
	tok := ExtractToken(headers)
	if tok != "mytoken123" {
		t.Errorf("got %q, want mytoken123", tok)
	}
}

func TestExtractToken_CaseInsensitiveKey(t *testing.T) {
	headers := map[string]string{"authorization": "Bearer mytoken123"}
	tok := ExtractToken(headers)
	if tok != "mytoken123" {
		t.Errorf("got %q, want mytoken123", tok)
	}
}

func TestExtractToken_CaseInsensitiveBearer(t *testing.T) {
	headers := map[string]string{"Authorization": "BEARER mytoken123"}
	tok := ExtractToken(headers)
	if tok != "mytoken123" {
		t.Errorf("got %q, want mytoken123", tok)
	}
}

func TestExtractToken_NoAuthHeader(t *testing.T) {
	headers := map[string]string{"Accept": "application/json"}
	tok := ExtractToken(headers)
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractToken_NotBearer(t *testing.T) {
	headers := map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}
	tok := ExtractToken(headers)
	if tok != "" {
		t.Errorf("expected empty for Basic auth, got %q", tok)
	}
}

func TestExtractToken_EmptyMap(t *testing.T) {
	tok := ExtractToken(map[string]string{})
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractToken_JWTToken(t *testing.T) {
	jwt := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.signature"
	headers := map[string]string{"Authorization": "Bearer " + jwt}
	tok := ExtractToken(headers)
	if tok != jwt {
		t.Errorf("got %q, want %q", tok, jwt)
	}
}

// ============================================================================
// ExtractRefreshTokenFromCookie
// ============================================================================

func TestExtractRefreshTokenFromCookie_SimpleRtoken(t *testing.T) {
	cookie := "rtokenN=12345|myrefreshtoken"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "myrefreshtoken" {
		t.Errorf("got %q, want myrefreshtoken", tok)
	}
}

func TestExtractRefreshTokenFromCookie_URLEncoded(t *testing.T) {
	// URL-encoded pipe: %7C
	cookie := "rtokenN=12345%7Cmyrefreshtoken"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "myrefreshtoken" {
		t.Errorf("got %q, want myrefreshtoken", tok)
	}
}

func TestExtractRefreshTokenFromCookie_MultipleCookies(t *testing.T) {
	cookie := "session=abc; other=xyz; rtokenN=99|secrettoken; foo=bar"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "secrettoken" {
		t.Errorf("got %q, want secrettoken", tok)
	}
}

func TestExtractRefreshTokenFromCookie_NoPipe(t *testing.T) {
	// No pipe — returns the whole value.
	cookie := "rtokenN=plainvalue"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "plainvalue" {
		t.Errorf("got %q, want plainvalue", tok)
	}
}

func TestExtractRefreshTokenFromCookie_Empty(t *testing.T) {
	tok := ExtractRefreshTokenFromCookie("")
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractRefreshTokenFromCookie_NoRtoken(t *testing.T) {
	cookie := "session=abc; other=xyz"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractRefreshTokenFromCookie_CaseInsensitive(t *testing.T) {
	cookie := "RTOKENN=12345|mytoken"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "mytoken" {
		t.Errorf("got %q, want mytoken", tok)
	}
}

func TestExtractRefreshTokenFromCookie_FallbackRegex(t *testing.T) {
	// Value without semicolons — exercises fallback regexp path.
	cookie := "rtokenX=777|regextoken"
	tok := ExtractRefreshTokenFromCookie(cookie)
	if tok != "regextoken" {
		t.Errorf("got %q, want regextoken", tok)
	}
}

// ============================================================================
// ExtractRefreshTokenFromCurlBody
// ============================================================================

func TestExtractRefreshTokenFromCurlBody_Nil(t *testing.T) {
	tok := ExtractRefreshTokenFromCurlBody(nil)
	if tok != "" {
		t.Errorf("expected empty for nil body, got %q", tok)
	}
}

func TestExtractRefreshTokenFromCurlBody_FormEncoded(t *testing.T) {
	body := "grant_type=refresh_token&refresh_token=myrefresh42"
	tok := ExtractRefreshTokenFromCurlBody(&body)
	if tok != "myrefresh42" {
		t.Errorf("got %q, want myrefresh42", tok)
	}
}

func TestExtractRefreshTokenFromCurlBody_JSON(t *testing.T) {
	body := `{"grant_type":"refresh_token","refresh_token":"jsonrefresh99"}`
	tok := ExtractRefreshTokenFromCurlBody(&body)
	if tok != "jsonrefresh99" {
		t.Errorf("got %q, want jsonrefresh99", tok)
	}
}

func TestExtractRefreshTokenFromCurlBody_NoRefreshToken(t *testing.T) {
	body := "grant_type=password&username=user&password=pass"
	tok := ExtractRefreshTokenFromCurlBody(&body)
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractRefreshTokenFromCurlBody_EmptyString(t *testing.T) {
	body := ""
	tok := ExtractRefreshTokenFromCurlBody(&body)
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractRefreshTokenFromCurlBody_JSONMissingField(t *testing.T) {
	body := `{"foo":"bar"}`
	tok := ExtractRefreshTokenFromCurlBody(&body)
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

// ============================================================================
// TokenString / RefreshTokenString
// ============================================================================

func TestTokenString_NilSession(t *testing.T) {
	if s := TokenString(nil); s != "" {
		t.Errorf("expected empty for nil session, got %q", s)
	}
}

func TestTokenString_NilToken(t *testing.T) {
	s := &Session{}
	if tok := TokenString(s); tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestTokenString_StringToken(t *testing.T) {
	s := &Session{Token: "mytoken"}
	if tok := TokenString(s); tok != "mytoken" {
		t.Errorf("got %q, want mytoken", tok)
	}
}

func TestTokenString_OtherType(t *testing.T) {
	s := &Session{Token: 42}
	if tok := TokenString(s); tok == "" {
		t.Error("expected non-empty for int token")
	}
}

func TestRefreshTokenString_NilSession(t *testing.T) {
	if s := RefreshTokenString(nil); s != "" {
		t.Errorf("expected empty for nil session, got %q", s)
	}
}

func TestRefreshTokenString_NilToken(t *testing.T) {
	s := &Session{}
	if tok := RefreshTokenString(s); tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestRefreshTokenString_StringToken(t *testing.T) {
	s := &Session{RefreshToken: "refresh42"}
	if tok := RefreshTokenString(s); tok != "refresh42" {
		t.Errorf("got %q, want refresh42", tok)
	}
}

func TestRefreshTokenString_OtherType(t *testing.T) {
	s := &Session{RefreshToken: 3.14}
	if tok := RefreshTokenString(s); tok == "" {
		t.Error("expected non-empty for float token")
	}
}

// ============================================================================
// CurlToSession (ApplyFromCurl + ParseCurl together)
// ============================================================================

func TestApplyFromCurl_TokenExtracted(t *testing.T) {
	cmd := `curl 'https://www.frisco.pl/api/v1/users/12345/cart' -H 'Authorization: Bearer beartok'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if TokenString(s) != "beartok" {
		t.Errorf("token: got %q, want beartok", TokenString(s))
	}
	if s.Headers["Authorization"] != "Bearer beartok" {
		t.Errorf("Authorization header: got %q", s.Headers["Authorization"])
	}
}

func TestApplyFromCurl_UserIDFromURL(t *testing.T) {
	cmd := `curl 'https://www.frisco.pl/api/v1/users/99999/orders'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if UserIDString(s) != "99999" {
		t.Errorf("user_id: got %q, want 99999", UserIDString(s))
	}
}

func TestApplyFromCurl_BaseURLUpdated(t *testing.T) {
	cmd := `curl 'https://staging.frisco.pl/api/v1/products'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if s.BaseURL != "https://staging.frisco.pl" {
		t.Errorf("base_url: got %q, want https://staging.frisco.pl", s.BaseURL)
	}
}

func TestApplyFromCurl_RefreshTokenFromCookie(t *testing.T) {
	cmd := `curl 'https://www.frisco.pl/api/auth/token' -H 'Cookie: rtokenN=42|cookierefresh'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if RefreshTokenString(s) != "cookierefresh" {
		t.Errorf("refresh_token: got %q, want cookierefresh", RefreshTokenString(s))
	}
}

func TestApplyFromCurl_RefreshTokenFromBody(t *testing.T) {
	cmd := `curl -X POST --data-raw 'grant_type=refresh_token&refresh_token=bodyrefresh' 'https://www.frisco.pl/api/auth/token'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if RefreshTokenString(s) != "bodyrefresh" {
		t.Errorf("refresh_token: got %q, want bodyrefresh", RefreshTokenString(s))
	}
}

func TestApplyFromCurl_NonAllowedHeadersFiltered(t *testing.T) {
	// User-Agent and X-Custom should not appear in session headers.
	cmd := `curl 'https://www.frisco.pl/api/v1/products' -H 'User-Agent: Mozilla/5.0' -H 'X-Custom: should-be-dropped' -H 'Accept: application/json'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if _, ok := s.Headers["User-Agent"]; ok {
		t.Error("User-Agent should have been filtered out")
	}
	if _, ok := s.Headers["X-Custom"]; ok {
		t.Error("X-Custom should have been filtered out")
	}
	if s.Headers["Accept"] != "application/json" {
		t.Errorf("Accept: got %q, want application/json", s.Headers["Accept"])
	}
}

func TestApplyFromCurl_AllowedHeadersKept(t *testing.T) {
	cmd := `curl 'https://www.frisco.pl/api/v1/products' ` +
		`-H 'x-api-version: 3' ` +
		`-H 'x-requested-with: XMLHttpRequest' ` +
		`-H 'origin: https://www.frisco.pl' ` +
		`-H 'referer: https://www.frisco.pl/kategoria/owoce'`
	c, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	s := defaultSession()
	ApplyFromCurl(s, c)
	if s.Headers["X-Api-Version"] != "3" {
		t.Errorf("X-Api-Version: got %q", s.Headers["X-Api-Version"])
	}
	if s.Headers["X-Requested-With"] != "XMLHttpRequest" {
		t.Errorf("X-Requested-With: got %q", s.Headers["X-Requested-With"])
	}
	if s.Headers["Origin"] != "https://www.frisco.pl" {
		t.Errorf("Origin: got %q", s.Headers["Origin"])
	}
	if s.Headers["Referer"] != "https://www.frisco.pl/kategoria/owoce" {
		t.Errorf("Referer: got %q", s.Headers["Referer"])
	}
}

// ============================================================================
// RedactedCopy (from show.go)
// ============================================================================

func TestRedactedCopy_NilSession(t *testing.T) {
	m := RedactedCopy(nil)
	if m != nil {
		t.Errorf("expected nil for nil session, got %v", m)
	}
}

func TestRedactedCopy_TokenRedacted(t *testing.T) {
	s := &Session{
		BaseURL: "https://www.frisco.pl",
		Token:   "supersecrettoken",
		Headers: map[string]string{},
	}
	m := RedactedCopy(s)
	if m["token"] != "***" {
		t.Errorf("token should be redacted, got %v", m["token"])
	}
}

func TestRedactedCopy_RefreshTokenRedacted(t *testing.T) {
	s := &Session{
		BaseURL:      "https://www.frisco.pl",
		RefreshToken: "myrefreshtoken",
		Headers:      map[string]string{},
	}
	m := RedactedCopy(s)
	if m["refresh_token"] != "***" {
		t.Errorf("refresh_token should be redacted, got %v", m["refresh_token"])
	}
}

func TestRedactedCopy_AuthorizationHeaderRedacted(t *testing.T) {
	s := &Session{
		BaseURL: "https://www.frisco.pl",
		Headers: map[string]string{"Authorization": "Bearer secret"},
	}
	m := RedactedCopy(s)
	headers, ok := m["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers not a map: %T", m["headers"])
	}
	if headers["Authorization"] != "***" {
		t.Errorf("Authorization header should be redacted, got %v", headers["Authorization"])
	}
}

func TestRedactedCopy_CookieHeaderRedacted(t *testing.T) {
	s := &Session{
		BaseURL: "https://www.frisco.pl",
		Headers: map[string]string{"Cookie": "rtokenN=12345|secret"},
	}
	m := RedactedCopy(s)
	headers, ok := m["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers not a map: %T", m["headers"])
	}
	if headers["Cookie"] != "***" {
		t.Errorf("Cookie header should be redacted, got %v", headers["Cookie"])
	}
}

func TestRedactedCopy_NonSensitiveHeadersPreserved(t *testing.T) {
	s := &Session{
		BaseURL: "https://www.frisco.pl",
		Headers: map[string]string{
			"Accept":       "application/json",
			"X-Api-Version": "3",
		},
	}
	m := RedactedCopy(s)
	headers, ok := m["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers not a map: %T", m["headers"])
	}
	if headers["Accept"] != "application/json" {
		t.Errorf("Accept should be preserved, got %v", headers["Accept"])
	}
	if headers["X-Api-Version"] != "3" {
		t.Errorf("X-Api-Version should be preserved, got %v", headers["X-Api-Version"])
	}
}

func TestRedactedCopy_NilTokenNotRedacted(t *testing.T) {
	s := &Session{
		BaseURL: "https://www.frisco.pl",
		Headers: map[string]string{},
	}
	m := RedactedCopy(s)
	// nil token should stay nil (not be replaced with "***")
	if tok, ok := m["token"]; ok && tok == "***" {
		t.Error("nil token should not be redacted to ***")
	}
}

func TestRedactedCopy_BaseURLPreserved(t *testing.T) {
	s := &Session{
		BaseURL: "https://www.frisco.pl",
		Token:   "tok",
		Headers: map[string]string{},
	}
	m := RedactedCopy(s)
	if m["base_url"] != "https://www.frisco.pl" {
		t.Errorf("base_url: got %v, want https://www.frisco.pl", m["base_url"])
	}
}

func TestRedactedCopy_OriginalSessionUnmodified(t *testing.T) {
	s := &Session{
		BaseURL:      "https://www.frisco.pl",
		Token:        "realtoken",
		RefreshToken: "realrefresh",
		Headers:      map[string]string{"Authorization": "Bearer realtoken"},
	}
	_ = RedactedCopy(s)
	if TokenString(s) != "realtoken" {
		t.Error("original session token was modified")
	}
	if RefreshTokenString(s) != "realrefresh" {
		t.Error("original session refresh_token was modified")
	}
	if s.Headers["Authorization"] != "Bearer realtoken" {
		t.Error("original session Authorization header was modified")
	}
}

// ============================================================================
// NormalizeHeaders
// ============================================================================

func TestNormalizeHeaders_EmptyMap(t *testing.T) {
	out := NormalizeHeaders(map[string]string{})
	if len(out) != 0 {
		t.Errorf("expected empty map, got %v", out)
	}
}

func TestNormalizeHeaders_CanonicalKeys(t *testing.T) {
	in := map[string]string{
		"authorization": "Bearer tok",
		"content-type":  "application/json",
		"accept":        "application/json",
	}
	out := NormalizeHeaders(in)
	if _, ok := out["Authorization"]; !ok {
		t.Error("Authorization key not canonicalized")
	}
	if _, ok := out["Content-Type"]; !ok {
		t.Error("Content-Type key not canonicalized")
	}
	if _, ok := out["Accept"]; !ok {
		t.Error("Accept key not canonicalized")
	}
}

func TestNormalizeHeaders_DeduplicatesCaseInsensitive(t *testing.T) {
	// Two variants of the same header; canonical casing should win.
	in := map[string]string{
		"authorization": "Bearer old",
		"Authorization": "Bearer new",
	}
	out := NormalizeHeaders(in)
	// Should have exactly one Authorization key with the canonical form.
	count := 0
	for k := range out {
		if strings.EqualFold(k, "authorization") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 authorization key after dedup, got %d", count)
	}
}

func TestNormalizeHeaders_XApiVersionCanonicalized(t *testing.T) {
	in := map[string]string{"x-api-version": "3"}
	out := NormalizeHeaders(in)
	if out["X-Api-Version"] != "3" {
		t.Errorf("X-Api-Version not canonicalized: %v", out)
	}
}

func TestNormalizeHeaders_UnknownKeyPreservedAsIs(t *testing.T) {
	in := map[string]string{"X-Custom-Header": "value"}
	out := NormalizeHeaders(in)
	if out["X-Custom-Header"] != "value" {
		t.Errorf("unknown key should be preserved as-is: %v", out)
	}
}

// ============================================================================
// UserIDString / RequireUserID
// ============================================================================

func TestUserIDString_NilSession(t *testing.T) {
	if s := UserIDString(nil); s != "" {
		t.Errorf("expected empty, got %q", s)
	}
}

func TestUserIDString_StringUserID(t *testing.T) {
	s := &Session{UserID: "12345"}
	if uid := UserIDString(s); uid != "12345" {
		t.Errorf("got %q, want 12345", uid)
	}
}

func TestUserIDString_Float64UserID(t *testing.T) {
	s := &Session{UserID: float64(99999)}
	if uid := UserIDString(s); uid != "99999" {
		t.Errorf("got %q, want 99999", uid)
	}
}

func TestRequireUserID_ExplicitTakesPriority(t *testing.T) {
	s := &Session{UserID: "11111"}
	uid, err := RequireUserID(s, "22222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != "22222" {
		t.Errorf("got %q, want 22222 (explicit should override session)", uid)
	}
}

func TestRequireUserID_FallsBackToSession(t *testing.T) {
	s := &Session{UserID: "33333"}
	uid, err := RequireUserID(s, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != "33333" {
		t.Errorf("got %q, want 33333", uid)
	}
}

func TestRequireUserID_ErrorWhenMissing(t *testing.T) {
	s := &Session{}
	_, err := RequireUserID(s, "")
	if err == nil {
		t.Error("expected error when user_id is missing")
	}
}
