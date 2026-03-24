package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func registerAccountXhrSessionHarAuthTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_profile",
		Description: "Fetch account profile (GET /users/{id}).",
	}, toolAccountProfile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_list",
		Description: "List shipping addresses (GET /users/{id}/addresses/shipping-addresses).",
	}, toolAccountAddressesList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_add",
		Description: "Add shipping address JSON (object or {shippingAddress:{...}}).",
	}, toolAccountAddressesAdd)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_update",
		Description: "Update shipping address by UUID (PUT). Body object or {shippingAddress:{...}}.",
	}, toolAccountAddressesUpdate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_delete",
		Description: "Delete shipping address by UUID.",
	}, toolAccountAddressesDelete)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_consents_update",
		Description: "Update account consents (PUT /users/{id}/consents).",
	}, toolAccountConsentsUpdate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_rules_accept",
		Description: "Accept account rules (PUT /users/{id}/rules).",
	}, toolAccountRulesAccept)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_vouchers",
		Description: "Fetch account vouchers (GET /users/{id}/vouchers).",
	}, toolAccountVouchers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_payments",
		Description: "Fetch account payment methods (GET /users/{id}/payments).",
	}, toolAccountPayments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_membership_cards",
		Description: "Fetch membership cards (GET /users/{id}/membership-cards).",
	}, toolAccountMembershipCards)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_membership_points",
		Description: "Fetch membership points history (GET /users/{id}/membership/points).",
	}, toolAccountMembershipPoints)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "xhr_list",
		Description: "List imported XHR endpoints from session (optional contains filter).",
	}, toolXHRList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "xhr_call",
		Description: "HTTP call with session auth (method, path_or_url, query, headers, data).",
	}, toolXHRCall)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_show",
		Description: "Current session with secrets redacted (same as CLI session show).",
	}, toolSessionShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_from_curl",
		Description: "Parse curl, ApplyFromCurl, Save (mirrors CLI session from-curl).",
	}, toolSessionFromCurl)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "auth_refresh_token",
		Description: "POST /app/commerce/connect/token with refresh_token grant.",
	}, toolAuthRefreshToken)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "har_import",
		Description: "Parse HAR for XHR, persist endpoints to session (mirrors CLI har import).",
	}, toolHarImport)
}

type accountAddressesListIn struct {
	UserID string `json:"user_id,omitempty" jsonschema:"optional; defaults to session user_id"`
}

type accountProfileIn struct {
	UserID string `json:"user_id,omitempty" jsonschema:"optional; defaults to session user_id"`
}

func toolAccountProfile(_ context.Context, _ *mcp.CallToolRequest, in accountProfileIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func toolAccountAddressesList(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesListIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountAddressesAddIn struct {
	UserID  string         `json:"user_id,omitempty"`
	Payload map[string]any `json:"payload"`
}

func toolAccountAddressesAdd(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesAddIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	body := wrapShippingAddressPayload(in.Payload)
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", uid)
	result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountAddressesUpdateIn struct {
	UserID    string         `json:"user_id,omitempty"`
	AddressID string         `json:"address_id"`
	Payload   map[string]any `json:"payload"`
}

func toolAccountAddressesUpdate(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesUpdateIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.AddressID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("address_id is required")
	}
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	body := wrapShippingAddressPayload(in.Payload)
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses/%s", uid, in.AddressID)
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountAddressesDeleteIn struct {
	UserID    string `json:"user_id,omitempty"`
	AddressID string `json:"address_id"`
}

func toolAccountAddressesDelete(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesDeleteIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.AddressID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("address_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses/%s", uid, in.AddressID)
	result, err := httpclient.RequestJSON(s, "DELETE", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func wrapShippingAddressPayload(data map[string]any) map[string]any {
	if _, has := data["shippingAddress"]; has {
		return data
	}
	return map[string]any{"shippingAddress": data}
}

type accountConsentsUpdateIn struct {
	UserID  string         `json:"user_id,omitempty"`
	Payload map[string]any `json:"payload"`
}

func toolAccountConsentsUpdate(_ context.Context, _ *mcp.CallToolRequest, in accountConsentsUpdateIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/consents", uid)
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       in.Payload,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountRulesAcceptIn struct {
	UserID  string         `json:"user_id,omitempty"`
	RuleIDs []string       `json:"rule_ids,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

func toolAccountRulesAccept(_ context.Context, _ *mcp.CallToolRequest, in accountRulesAcceptIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	body := in.Payload
	if len(body) == 0 {
		if len(in.RuleIDs) == 0 {
			return nil, mcpCPFriscoToolOut{}, errors.New("provide payload or rule_ids")
		}
		body = map[string]any{"acceptedRules": in.RuleIDs}
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/rules", uid)
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountVouchersIn struct {
	UserID string `json:"user_id,omitempty"`
}

func toolAccountVouchers(_ context.Context, _ *mcp.CallToolRequest, in accountVouchersIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/vouchers", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountPaymentsIn struct {
	UserID string `json:"user_id,omitempty"`
}

func toolAccountPayments(_ context.Context, _ *mcp.CallToolRequest, in accountPaymentsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/payments", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountMembershipCardsIn struct {
	UserID string `json:"user_id,omitempty"`
}

func toolAccountMembershipCards(_ context.Context, _ *mcp.CallToolRequest, in accountMembershipCardsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership-cards", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type accountMembershipPointsIn struct {
	UserID    string `json:"user_id,omitempty"`
	PageIndex int    `json:"page_index,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
}

func toolAccountMembershipPoints(_ context.Context, _ *mcp.CallToolRequest, in accountMembershipPointsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	pageIndex := in.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 25
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership/points", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{
		Query: []string{
			fmt.Sprintf("pageIndex=%d", pageIndex),
			fmt.Sprintf("pageSize=%d", pageSize),
		},
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type xhrListIn struct {
	Contains string `json:"contains,omitempty" jsonschema:"substring filter on path template or method"`
}

func toolXHRList(_ context.Context, _ *mcp.CallToolRequest, in xhrListIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	endpoints := s.Endpoints
	if len(endpoints) == 0 {
		return mcpCPWrapFriscoResult(map[string]any{
			"endpoints": []session.Endpoint{},
			"total":     0,
			"message":   "No endpoints in session. Run har_import first.",
		})
	}
	filtered := endpoints
	if needle := strings.TrimSpace(in.Contains); needle != "" {
		n := strings.ToLower(needle)
		var next []session.Endpoint
		for _, ep := range endpoints {
			if strings.Contains(strings.ToLower(ep.PathTemplate), n) ||
				strings.Contains(strings.ToLower(ep.Method), n) {
				next = append(next, ep)
			}
		}
		filtered = next
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"endpoints": filtered,
		"total":     len(filtered),
	})
}

type xhrCallIn struct {
	Method     string   `json:"method"`
	PathOrURL  string   `json:"path_or_url"`
	Query      []string `json:"query,omitempty"`
	Header     []string `json:"header,omitempty" jsonschema:"Key: Value per element"`
	Data       string   `json:"data,omitempty"`
	DataFormat string   `json:"data_format,omitempty" jsonschema:"auto, json, form, raw"`
}

func toolXHRCall(_ context.Context, _ *mcp.CallToolRequest, in xhrCallIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	method := strings.TrimSpace(in.Method)
	pathOrURL := strings.TrimSpace(in.PathOrURL)
	if method == "" || pathOrURL == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("method and path_or_url are required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	payload, err := mcpAXSHParseJSONOrKV(in.Data)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	extra := map[string]string{}
	for _, h := range in.Header {
		idx := strings.IndexByte(h, ':')
		if idx < 0 {
			return nil, mcpCPFriscoToolOut{}, fmt.Errorf("bad header %q: expected Key: Value", h)
		}
		extra[strings.TrimSpace(h[:idx])] = strings.TrimSpace(h[idx+1:])
	}
	opts := httpclient.RequestOpts{
		Query:        in.Query,
		ExtraHeaders: extra,
	}
	if payload != nil {
		format := in.DataFormat
		if format == "" || format == "auto" {
			trim := strings.TrimSpace(in.Data)
			switch {
			case strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "["):
				format = "json"
			case strings.Contains(trim, "="):
				format = "form"
			default:
				format = "raw"
			}
		}
		switch format {
		case "json":
			opts.Data = payload
			opts.DataFormat = httpclient.FormatJSON
		case "form":
			opts.Data = payload
			opts.DataFormat = httpclient.FormatForm
		case "raw":
			strPayload, ok := payload.(string)
			if !ok {
				return nil, mcpCPFriscoToolOut{}, errors.New("data_format raw requires a string body")
			}
			opts.Data = strPayload
			opts.DataFormat = httpclient.FormatRaw
		default:
			return nil, mcpCPFriscoToolOut{}, fmt.Errorf("unsupported data_format: %s", format)
		}
	}
	result, err := httpclient.RequestJSON(s, method, pathOrURL, opts)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type sessionShowIn struct{}

func toolSessionShow(_ context.Context, _ *mcp.CallToolRequest, _ sessionShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(session.RedactedCopy(s))
}

type sessionFromCurlIn struct {
	Curl string `json:"curl"`
}

func toolSessionFromCurl(_ context.Context, _ *mcp.CallToolRequest, in sessionFromCurlIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.Curl) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("curl is required")
	}
	cd, err := session.ParseCurl(in.Curl)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	session.ApplyFromCurl(s, cd)
	if err := session.Save(s); err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":         true,
		"base_url":      s.BaseURL,
		"user_id":       s.UserID,
		"token_saved":   mcpAXSHTokenSaved(s),
		"headers_saved": mcpAXSHHeaderKeysSorted(s.Headers),
	})
}

type authRefreshTokenIn struct {
	RefreshToken string `json:"refresh_token,omitempty" jsonschema:"optional; else session refresh_token"`
}

func toolAuthRefreshToken(_ context.Context, _ *mcp.CallToolRequest, in authRefreshTokenIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	rt := strings.TrimSpace(in.RefreshToken)
	if rt == "" {
		rt = session.RefreshTokenString(s)
	}
	if rt == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("missing refresh_token (argument or session)")
	}
	payload := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": rt,
	}
	result, err := httpclient.RequestJSON(s, "POST", "/app/commerce/connect/token", httpclient.RequestOpts{
		Data:       payload,
		DataFormat: httpclient.FormatForm,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if m, ok := result.(map[string]any); ok {
		expiresIn := m["expires_in"]
		if at, ok := mcpAXSHStringField(m["access_token"]); ok && at != "" {
			s.Token = at
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			s.Headers["Authorization"] = "Bearer " + at
		}
		if nr, ok := mcpAXSHStringField(m["refresh_token"]); ok && nr != "" {
			s.RefreshToken = nr
		}
		if err := session.Save(s); err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(map[string]any{
			"saved":               true,
			"token_saved":         mcpAXSHTokenSaved(s),
			"refresh_token_saved": session.RefreshTokenString(s) != "",
			"expires_in":          expiresIn,
		})
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":               false,
		"token_saved":         mcpAXSHTokenSaved(s),
		"refresh_token_saved": session.RefreshTokenString(s) != "",
		"message":             "Unexpected token endpoint payload shape; session not updated.",
	})
}

type harImportIn struct {
	Path string `json:"path,omitempty" jsonschema:"HAR path; default session.DefaultHARPath"`
}

func toolHarImport(_ context.Context, _ *mcp.CallToolRequest, in harImportIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	p := strings.TrimSpace(in.Path)
	if p == "" {
		p = session.DefaultHARPath()
	}
	endpoints, err := session.ParseHarXHR(p)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	s.Endpoints = endpoints
	s.HarPath = p
	for _, ep := range endpoints {
		if uid := session.ExtractUserID(ep.URL); uid != "" {
			s.UserID = uid
			break
		}
	}
	if err := session.Save(s); err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"imported":        len(endpoints),
		"har_path":        p,
		"session_user_id": session.UserIDString(s),
	})
}

func mcpAXSHParseJSONOrKV(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return raw, nil
	}
	if len(vals) == 0 {
		return raw, nil
	}
	m := make(map[string]any, len(vals))
	for k, vs := range vals {
		if len(vs) == 1 {
			m[k] = vs[0]
		} else {
			m[k] = vs
		}
	}
	return m, nil
}

func mcpAXSHStringField(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		return s, s != ""
	}
}

func mcpAXSHTokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

func mcpAXSHHeaderKeysSorted(h map[string]string) []string {
	if len(h) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
