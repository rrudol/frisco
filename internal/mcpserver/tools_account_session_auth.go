package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func registerAccountSessionAuthTools(server *mcp.Server) {
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
		Name:        "session_show",
		Description: "Current session with secrets redacted (same as CLI session show).",
	}, toolSessionShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_from_curl",
		Description: "Parse curl, ApplyFromCurl, Save (mirrors CLI session from-curl).",
	}, toolSessionFromCurl)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_refresh_token",
		Description: "POST /app/commerce/connect/token with refresh_token grant.",
	}, toolAuthRefreshToken)
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
		"token_saved":   mcpASATokenSaved(s),
		"headers_saved": mcpASAHeaderKeysSorted(s.Headers),
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
		if at, ok := mcpASAStringField(m["access_token"]); ok && at != "" {
			s.Token = at
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			s.Headers["Authorization"] = "Bearer " + at
		}
		if nr, ok := mcpASAStringField(m["refresh_token"]); ok && nr != "" {
			s.RefreshToken = nr
		}
		if err := session.Save(s); err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(map[string]any{
			"saved":               true,
			"token_saved":         mcpASATokenSaved(s),
			"refresh_token_saved": session.RefreshTokenString(s) != "",
			"expires_in":          expiresIn,
		})
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":               false,
		"token_saved":         mcpASATokenSaved(s),
		"refresh_token_saved": session.RefreshTokenString(s) != "",
		"message":             "Unexpected token endpoint payload shape; session not updated.",
	})
}

func mcpASAStringField(v any) (string, bool) {
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

func mcpASATokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

func mcpASAHeaderKeysSorted(h map[string]string) []string {
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
