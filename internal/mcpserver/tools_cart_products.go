package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
)

// mcpCPFriscoToolOut is the structured tool result envelope for cart/product tools.
type mcpCPFriscoToolOut struct {
	OK   bool            `json:"ok" jsonschema:"true when the Frisco API call completed without a transport error"`
	Data json.RawMessage `json:"data,omitempty" jsonschema:"JSON body returned by Frisco (object, array, or scalar)"`
}

func registerCartAndProductsTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cart_show",
		Description: "Fetch the current shopping cart for a Frisco user (GET /app/commerce/api/v1/users/{id}/cart). Uses ~/.frisco-cli/session.json unless user_id is set.",
	}, mcpCPCartShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cart_add",
		Description: "Add or set product quantity in the cart (PUT cart with products[]. Same as frisco cart add.",
	}, mcpCPCartAdd)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cart_remove",
		Description: "Remove a product from the cart by setting quantity to 0 (PUT cart). Same as frisco cart remove.",
	}, mcpCPCartRemove)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "products_search",
		Description: "Search the product catalog (GET .../offer/products/query). Optional category_id narrows by Frisco categoryId (same as frisco products search --category-id). Mirrors frisco products search.",
	}, mcpCPProductsSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "products_by_ids",
		Description: "Fetch products by repeated productIds query params. Mirrors frisco products by-ids.",
	}, mcpCPProductsByIDs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "products_nutrition",
		Description: "Fetch product content from /app/content/api/v1/products/get/{id}. By default returns extracted nutrition block if found; set raw true for full API JSON.",
	}, mcpCPProductsNutrition)
}

type mcpCPCartShowIn struct {
	UserID string `json:"user_id,omitempty" jsonschema:"Frisco user id; falls back to session user_id"`
}

func mcpCPCartShow(_ context.Context, _ *mcp.CallToolRequest, in mcpCPCartShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type mcpCPCartAddIn struct {
	ProductID string `json:"product_id" jsonschema:"Frisco product id"`
	Quantity  *int   `json:"quantity,omitempty" jsonschema:"defaults to 1 when omitted"`
	UserID    string `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPCartAdd(_ context.Context, _ *mcp.CallToolRequest, in mcpCPCartAddIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	qty := 1
	if in.Quantity != nil {
		qty = *in.Quantity
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	body := map[string]any{
		"products": []any{
			map[string]any{"productId": in.ProductID, "quantity": qty},
		},
	}
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type mcpCPCartRemoveIn struct {
	ProductID string `json:"product_id" jsonschema:"Frisco product id"`
	UserID    string `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPCartRemove(_ context.Context, _ *mcp.CallToolRequest, in mcpCPCartRemoveIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	body := map[string]any{
		"products": []any{
			map[string]any{"productId": in.ProductID, "quantity": 0},
		},
	}
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type mcpCPProductsSearchIn struct {
	Search          string  `json:"search" jsonschema:"search phrase (purpose=Listing)"`
	CategoryID      string  `json:"category_id,omitempty" jsonschema:"optional Frisco categoryId to narrow results (e.g. 18703 Warzywa i owoce)"`
	PageIndex       *int    `json:"page_index,omitempty" jsonschema:"1-based page index; default 1"`
	PageSize        *int    `json:"page_size,omitempty" jsonschema:"default 84"`
	DeliveryMethod  *string `json:"delivery_method,omitempty" jsonschema:"default Van"`
	UserID          string  `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPProductsSearch(_ context.Context, _ *mcp.CallToolRequest, in mcpCPProductsSearchIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.Search) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("search is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	pageIndex := 1
	if in.PageIndex != nil {
		pageIndex = *in.PageIndex
	}
	pageSize := 84
	if in.PageSize != nil {
		pageSize = *in.PageSize
	}
	deliveryMethod := "Van"
	if in.DeliveryMethod != nil && *in.DeliveryMethod != "" {
		deliveryMethod = *in.DeliveryMethod
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products/query", uid)
	q := []string{
		"purpose=Listing",
		fmt.Sprintf("pageIndex=%d", pageIndex),
		fmt.Sprintf("search=%s", in.Search),
		"includeFacets=true",
		fmt.Sprintf("deliveryMethod=%s", deliveryMethod),
		fmt.Sprintf("pageSize=%d", pageSize),
		"language=pl",
		"disableAutocorrect=false",
	}
	if strings.TrimSpace(in.CategoryID) != "" {
		q = append(q, fmt.Sprintf("categoryId=%s", strings.TrimSpace(in.CategoryID)))
	}
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type mcpCPProductsByIDsIn struct {
	ProductIDs []string `json:"product_ids" jsonschema:"list of Frisco product ids"`
	UserID     string   `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPProductsByIDs(_ context.Context, _ *mcp.CallToolRequest, in mcpCPProductsByIDsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.ProductIDs) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_ids must contain at least one id")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products", uid)
	var q []string
	for _, pid := range in.ProductIDs {
		q = append(q, fmt.Sprintf("productIds=%s", pid))
	}
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type mcpCPProductsNutritionIn struct {
	ProductID string `json:"product_id" jsonschema:"Frisco product id"`
	Raw       bool   `json:"raw,omitempty" jsonschema:"if true, return full API JSON; default false"`
}

func mcpCPProductsNutrition(_ context.Context, _ *mcp.CallToolRequest, in mcpCPProductsNutritionIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/content/api/v1/products/get/%s", in.ProductID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if in.Raw {
		return mcpCPWrapFriscoResult(result)
	}
	nutrition := shared.ExtractNutritionBlock(result)
	if nutrition == nil {
		out := map[string]any{
			"productId": in.ProductID,
			"message":   "No explicit nutrition block in this response. Use raw=true for the full API JSON.",
		}
		return mcpCPWrapFriscoResult(out)
	}
	out := map[string]any{
		"productId":  in.ProductID,
		"nutrition":  nutrition,
		"sourcePath": "/app/content/api/v1/products/get/{id}",
	}
	return mcpCPWrapFriscoResult(out)
}

func mcpCPWrapFriscoResult(v any) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	text := string(raw)
	const mcpCPMaxSummaryRunes = 8000
	if len(text) > mcpCPMaxSummaryRunes {
		text = text[:mcpCPMaxSummaryRunes] + "...[truncated]"
	}
	res := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
	return res, mcpCPFriscoToolOut{OK: true, Data: raw}, nil
}

