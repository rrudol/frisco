package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

// RegisterOrdersAndReservationTools registers orders_* and reservation_* MCP tools.
func RegisterOrdersAndReservationTools(server *mcp.Server) {
	registerOrdersAndReservationTools(server)
}

func registerOrdersAndReservationTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_list",
		Description: "List user orders (GET /app/commerce/api/v1/users/{user}/orders). Mirrors frisco orders list CLI.",
	}, orToolOrdersList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_details",
		Description: "Order details (GET .../users/{user}/orders/{orderId}). Mirrors frisco orders get.",
	}, orToolOrdersDetails)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_delivery",
		Description: "Order delivery details (GET .../users/{user}/orders/{orderId}/delivery). Mirrors frisco orders delivery.",
	}, orToolOrdersDelivery)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_payments",
		Description: "Order payments details (GET .../users/{user}/orders/{orderId}/payments). Mirrors frisco orders payments.",
	}, orToolOrdersPayments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_delivery_options",
		Description: "Delivery and payment options by postcode. Mirrors frisco reservation delivery-options.",
	}, orToolReservationDeliveryOptions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_calendar",
		Description: "Calendar data for shipping address; optional date. Mirrors frisco reservation calendar.",
	}, orToolReservationCalendar)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_slots",
		Description: "Available delivery slots for consecutive days. Mirrors frisco reservation slots.",
	}, orToolReservationSlots)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_reserve",
		Description: "Reserve a cart delivery window by date and HH:MM range. Mirrors frisco reservation reserve.",
	}, orToolReservationReserve)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_plan",
		Description: "Plan cart reservation from payload JSON object. Mirrors frisco reservation plan.",
	}, orToolReservationPlan)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_cancel",
		Description: "Cancel active cart reservation (DELETE .../cart/reservation). Mirrors frisco reservation cancel.",
	}, orToolReservationCancel)
}

// --- Input / output types (orders + reservation, unique or* prefix) ---

// orOrdersListIn is the input type for the orders_list tool.
type orOrdersListIn struct {
	UserID    string `json:"user_id,omitempty" jsonschema:"description=Optional override; else session user_id"`
	PageIndex int    `json:"page_index,omitempty" jsonschema:"description=1-based page index, default 1"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"description=Page size, default 10"`
	AllPages  bool   `json:"all_pages,omitempty" jsonschema:"description=Fetch every page until empty or cap"`
	Raw       bool   `json:"raw,omitempty" jsonschema:"description=If true, only api_response (no compact summary)"`
}

// orOrdersListOut is the output type for the orders_list tool.
type orOrdersListOut struct {
	APIResponse any              `json:"api_response" jsonschema:"description=Raw or aggregated API JSON"`
	Summary     map[string]any   `json:"summary,omitempty"`
	Orders      []map[string]any `json:"orders,omitempty"`
}

// orOrdersDetailsIn is the input type for the orders_details tool.
type orOrdersDetailsIn struct {
	UserID  string `json:"user_id,omitempty"`
	OrderID string `json:"order_id" jsonschema:"description=Order id"`
}

// orOrdersDetailsOut is the output type for the orders_details tool.
type orOrdersDetailsOut struct {
	APIResponse any `json:"api_response"`
}

// orOrdersDeliveryIn is the input type for the orders_delivery tool.
type orOrdersDeliveryIn struct {
	UserID  string `json:"user_id,omitempty"`
	OrderID string `json:"order_id" jsonschema:"description=Order id"`
}

// orOrdersDeliveryOut is the output type for the orders_delivery tool.
type orOrdersDeliveryOut struct {
	APIResponse any `json:"api_response"`
}

// orOrdersPaymentsIn is the input type for the orders_payments tool.
type orOrdersPaymentsIn struct {
	UserID  string `json:"user_id,omitempty"`
	OrderID string `json:"order_id" jsonschema:"description=Order id"`
}

// orOrdersPaymentsOut is the output type for the orders_payments tool.
type orOrdersPaymentsOut struct {
	APIResponse any `json:"api_response"`
}

// orReservationDeliveryOptionsIn is the input type for the reservation_delivery_options tool.
type orReservationDeliveryOptionsIn struct {
	Postcode string `json:"postcode" jsonschema:"description=postcode, e.g. 00-001"`
}

// orReservationDeliveryOptionsOut is the output type for the reservation_delivery_options tool.
type orReservationDeliveryOptionsOut struct {
	APIResponse any `json:"api_response"`
}

// orReservationCalendarIn is the input type for the reservation_calendar tool.
type orReservationCalendarIn struct {
	UserID          string         `json:"user_id,omitempty"`
	ShippingAddress map[string]any `json:"shipping_address" jsonschema:"description=shipping address object"`
	Date            string         `json:"date,omitempty" jsonschema:"description=optional YYYY-M-D or YYYY-MM-DD"`
}

// orReservationCalendarOut is the output type for the reservation_calendar tool.
type orReservationCalendarOut struct {
	APIResponse any `json:"api_response"`
}

// orReservationSlotsIn is the input type for the reservation_slots tool.
type orReservationSlotsIn struct {
	UserID           string         `json:"user_id,omitempty"`
	Days             int            `json:"days,omitempty" jsonschema:"description=Consecutive days to check, default 3"`
	StartDate        string         `json:"start_date,omitempty" jsonschema:"description=YYYY-MM-DD, default today"`
	ShippingAddress  map[string]any `json:"shipping_address,omitempty" jsonschema:"description=Optional; else load default from account"`
	Raw              bool           `json:"raw,omitempty"`
}

// orReservationSlotsOut is the output type for the reservation_slots tool.
type orReservationSlotsOut struct {
	APIByDate map[string]any   `json:"api_by_date,omitempty" jsonschema:"description=Per-day raw API payloads keyed by YYYY-MM-DD"`
	Days      []map[string]any `json:"days,omitempty" jsonschema:"description=Pretty slots per day when raw is false"`
}

// orReservationReserveIn is the input type for the reservation_reserve tool.
type orReservationReserveIn struct {
	UserID          string         `json:"user_id,omitempty"`
	Date            string         `json:"date" jsonschema:"description=YYYY-MM-DD"`
	FromTime        string         `json:"from_time" jsonschema:"description=HH:MM window start"`
	ToTime          string         `json:"to_time" jsonschema:"description=HH:MM window end"`
	ShippingAddress map[string]any `json:"shipping_address,omitempty"`
}

// orReservationReserveOut is the output type for the reservation_reserve tool.
type orReservationReserveOut struct {
	Reserved    bool           `json:"reserved"`
	Slot        map[string]any `json:"slot,omitempty"`
	APIResponse any            `json:"api_response"`
}

// orReservationCancelIn is the input type for the reservation_cancel tool.
type orReservationCancelIn struct {
	UserID string `json:"user_id,omitempty"`
}

// orReservationCancelOut is the output type for the reservation_cancel tool.
type orReservationCancelOut struct {
	APIResponse any `json:"api_response"`
}

// orReservationPlanIn is the input type for the reservation_plan tool.
type orReservationPlanIn struct {
	UserID  string         `json:"user_id,omitempty"`
	Payload map[string]any `json:"payload" jsonschema:"description=reservation payload object"`
}

// orReservationPlanOut is the output type for the reservation_plan tool.
type orReservationPlanOut struct {
	APIResponse any `json:"api_response"`
}

// --- Handlers ---

func orToolOrdersList(ctx context.Context, req *mcp.CallToolRequest, in orOrdersListIn) (*mcp.CallToolResult, orOrdersListOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, orOrdersListOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orOrdersListOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders", uid)
	pageIndex := in.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	var result any
	if in.AllPages {
		var allItems []map[string]any
		pi := pageIndex
		for {
			q := []string{
				fmt.Sprintf("pageIndex=%d", pi),
				fmt.Sprintf("pageSize=%d", pageSize),
			}
			payload, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return nil, orOrdersListOut{}, err
			}
			items := orExtractOrdersList(payload)
			if len(items) == 0 {
				break
			}
			allItems = append(allItems, items...)
			if len(items) < pageSize {
				break
			}
			pi++
			if pi-pageIndex > 100 {
				break
			}
		}
		result = allItems
	} else {
		q := []string{
			fmt.Sprintf("pageIndex=%d", pageIndex),
			fmt.Sprintf("pageSize=%d", pageSize),
		}
		result, err = httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
		if err != nil {
			return nil, orOrdersListOut{}, err
		}
	}
	out := orOrdersListOut{APIResponse: result}
	if in.Raw {
		return nil, out, nil
	}
	items := orExtractOrdersList(result)
	var compact []map[string]any
	for _, order := range items {
		id := order["id"]
		if id == nil {
			id = order["orderId"]
		}
		st := order["status"]
		if st == nil {
			st = order["orderStatus"]
		}
		row := map[string]any{
			"id":        id,
			"status":    st,
			"createdAt": orExtractOrderDatetime(order),
		}
		if t := orExtractOrderTotal(order); t != nil {
			row["totalPLN"] = math.Round(*t*100) / 100
		} else {
			row["totalPLN"] = nil
		}
		compact = append(compact, row)
	}
	var totalVals []float64
	for _, x := range compact {
		if v, ok := x["totalPLN"].(float64); ok {
			totalVals = append(totalVals, v)
		}
	}
	summary := map[string]any{"count": len(compact)}
	if len(totalVals) > 0 {
		var sum float64
		for _, v := range totalVals {
			sum += v
		}
		summary["sumPLN"] = math.Round(sum*100) / 100
		summary["avgPLN"] = math.Round(sum/float64(len(totalVals))*100) / 100
	} else {
		summary["sumPLN"] = nil
		summary["avgPLN"] = nil
	}
	out.Summary = summary
	out.Orders = compact
	return nil, out, nil
}

func orToolOrdersDetails(ctx context.Context, req *mcp.CallToolRequest, in orOrdersDetailsIn) (*mcp.CallToolResult, orOrdersDetailsOut, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return nil, orOrdersDetailsOut{}, fmt.Errorf("order_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orOrdersDetailsOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orOrdersDetailsOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s", uid, in.OrderID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orOrdersDetailsOut{}, err
	}
	return nil, orOrdersDetailsOut{APIResponse: result}, nil
}

func orToolOrdersDelivery(ctx context.Context, req *mcp.CallToolRequest, in orOrdersDeliveryIn) (*mcp.CallToolResult, orOrdersDeliveryOut, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return nil, orOrdersDeliveryOut{}, fmt.Errorf("order_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orOrdersDeliveryOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orOrdersDeliveryOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/delivery", uid, in.OrderID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orOrdersDeliveryOut{}, err
	}
	return nil, orOrdersDeliveryOut{APIResponse: result}, nil
}

func orToolOrdersPayments(ctx context.Context, req *mcp.CallToolRequest, in orOrdersPaymentsIn) (*mcp.CallToolResult, orOrdersPaymentsOut, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return nil, orOrdersPaymentsOut{}, fmt.Errorf("order_id is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orOrdersPaymentsOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orOrdersPaymentsOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/payments", uid, in.OrderID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orOrdersPaymentsOut{}, err
	}
	return nil, orOrdersPaymentsOut{APIResponse: result}, nil
}

func orToolReservationDeliveryOptions(ctx context.Context, req *mcp.CallToolRequest, in orReservationDeliveryOptionsIn) (*mcp.CallToolResult, orReservationDeliveryOptionsOut, error) {
	if strings.TrimSpace(in.Postcode) == "" {
		return nil, orReservationDeliveryOptionsOut{}, fmt.Errorf("postcode is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orReservationDeliveryOptionsOut{}, err
	}
	result, err := httpclient.RequestJSON(s, "GET", "/app/commerce/api/v1/calendar/delivery-payment", httpclient.RequestOpts{
		Query: []string{"postcode=" + in.Postcode},
	})
	if err != nil {
		return nil, orReservationDeliveryOptionsOut{}, err
	}
	return nil, orReservationDeliveryOptionsOut{APIResponse: result}, nil
}

func orToolReservationCalendar(ctx context.Context, req *mcp.CallToolRequest, in orReservationCalendarIn) (*mcp.CallToolResult, orReservationCalendarOut, error) {
	if len(in.ShippingAddress) == 0 {
		return nil, orReservationCalendarOut{}, fmt.Errorf("shipping_address is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orReservationCalendarOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orReservationCalendarOut{}, err
	}
	body := map[string]any{"shippingAddress": in.ShippingAddress}

	path := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van", uid)
	if strings.TrimSpace(in.Date) != "" {
		y, m, d, err := parseCalendarDate(in.Date)
		if err != nil {
			return nil, orReservationCalendarOut{}, err
		}
		path = fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d", uid, y, m, d)
	}
	result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationCalendarOut{}, err
	}
	return nil, orReservationCalendarOut{APIResponse: result}, nil
}

func orToolReservationSlots(ctx context.Context, req *mcp.CallToolRequest, in orReservationSlotsIn) (*mcp.CallToolResult, orReservationSlotsOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, orReservationSlotsOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orReservationSlotsOut{}, err
	}
	days := in.Days
	if days <= 0 {
		days = 3
	}
	var shippingAddress map[string]any
	if len(in.ShippingAddress) > 0 {
		shippingAddress = in.ShippingAddress
	} else {
		shippingAddress, err = orGetShippingAddressFromAccount(s, uid)
		if err != nil {
			return nil, orReservationSlotsOut{}, err
		}
	}
	var baseDate time.Time
	if in.StartDate != "" {
		baseDate, err = time.Parse("2006-01-02", in.StartDate)
		if err != nil {
			return nil, orReservationSlotsOut{}, err
		}
	} else {
		baseDate = time.Now().Truncate(24 * time.Hour)
	}
	allDays := map[string]any{}
	var pretty []map[string]any
	for i := 0; i < days; i++ {
		d := baseDate.AddDate(0, 0, i)
		calPath := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d",
			uid, d.Year(), int(d.Month()), d.Day())
		dayData, err := httpclient.RequestJSON(s, "POST", calPath, httpclient.RequestOpts{
			Data:       map[string]any{"shippingAddress": shippingAddress},
			DataFormat: httpclient.FormatJSON,
		})
		if err != nil {
			return nil, orReservationSlotsOut{}, err
		}
		dayKey := d.Format("2006-01-02")
		allDays[dayKey] = dayData
		pretty = append(pretty, map[string]any{
			"date":  dayKey,
			"slots": orExtractDeliveryWindows(dayData),
		})
	}
	out := orReservationSlotsOut{APIByDate: allDays}
	if in.Raw {
		return nil, out, nil
	}
	out.Days = pretty
	return nil, out, nil
}

func orToolReservationReserve(ctx context.Context, req *mcp.CallToolRequest, in orReservationReserveIn) (*mcp.CallToolResult, orReservationReserveOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	targetDate, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	var shippingAddress map[string]any
	if len(in.ShippingAddress) > 0 {
		shippingAddress = in.ShippingAddress
	} else {
		shippingAddress, err = orGetShippingAddressFromAccount(s, uid)
		if err != nil {
			return nil, orReservationReserveOut{}, err
		}
	}
	calPath := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d",
		uid, targetDate.Year(), int(targetDate.Month()), targetDate.Day())
	dayData, err := httpclient.RequestJSON(s, "POST", calPath, httpclient.RequestOpts{
		Data:       map[string]any{"shippingAddress": shippingAddress},
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	windows := orExtractReservableWindows(dayData)
	if len(windows) == 0 {
		return nil, orReservationReserveOut{}, fmt.Errorf("no reservable slots for the given date")
	}
	var selected map[string]any
	var possible []string
	for _, w := range windows {
		startsAt := fmt.Sprint(w["startsAt"])
		endsAt := fmt.Sprint(w["endsAt"])
		d1, h1 := orDateAndHHMMFromISO(startsAt)
		d2, h2 := orDateAndHHMMFromISO(endsAt)
		if d1 == in.Date && d2 == in.Date {
			possible = append(possible, fmt.Sprintf("%s-%s", h1, h2))
		}
		if d1 == in.Date && d2 == in.Date && h1 == in.FromTime && h2 == in.ToTime {
			selected = w
			break
		}
	}
	if selected == nil {
		return nil, orReservationReserveOut{}, fmt.Errorf("slot %s-%s not found for %s; available: %s",
			in.FromTime, in.ToTime, in.Date, strings.Join(possible, ", "))
	}
	payload := map[string]any{
		"extendedRange":   nil,
		"deliveryWindow":  selected,
		"shippingAddress": shippingAddress,
	}
	reservePath := fmt.Sprintf("/app/commerce/api/v2/users/%s/cart/reservation", uid)
	result, err := httpclient.RequestJSON(s, "POST", reservePath, httpclient.RequestOpts{
		Data:       payload,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	out := orReservationReserveOut{
		Reserved: true,
		Slot: map[string]any{
			"startsAt":       selected["startsAt"],
			"endsAt":         selected["endsAt"],
			"deliveryMethod": selected["deliveryMethod"],
			"warehouse":      selected["warehouse"],
		},
		APIResponse: result,
	}
	return nil, out, nil
}

func orToolReservationCancel(ctx context.Context, req *mcp.CallToolRequest, in orReservationCancelIn) (*mcp.CallToolResult, orReservationCancelOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, orReservationCancelOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orReservationCancelOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart/reservation", uid)
	result, err := httpclient.RequestJSON(s, "DELETE", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orReservationCancelOut{}, err
	}
	return nil, orReservationCancelOut{APIResponse: result}, nil
}

func orToolReservationPlan(ctx context.Context, req *mcp.CallToolRequest, in orReservationPlanIn) (*mcp.CallToolResult, orReservationPlanOut, error) {
	if len(in.Payload) == 0 {
		return nil, orReservationPlanOut{}, fmt.Errorf("payload is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orReservationPlanOut{}, err
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, orReservationPlanOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v2/users/%s/cart/reservation", uid)
	result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:       in.Payload,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationPlanOut{}, err
	}
	return nil, orReservationPlanOut{APIResponse: result}, nil
}

// --- Helpers (logic aligned with internal/commands orders + reservation) ---

// orExtractOrdersList extracts an orders slice from various API response shapes.
func orExtractOrdersList(payload any) []map[string]any {
	switch p := payload.(type) {
	case []map[string]any:
		return p
	case []any:
		var out []map[string]any
		for _, x := range p {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"items", "orders", "results", "data"} {
			if v, ok := p[key].([]any); ok {
				var out []map[string]any
				for _, x := range v {
					if m, ok := x.(map[string]any); ok {
						out = append(out, m)
					}
				}
				return out
			}
		}
	}
	return nil
}

// orExtractOrderDatetime returns the first non-empty date/time string found in an order map.
func orExtractOrderDatetime(order map[string]any) string {
	for _, key := range []string{"createdAt", "created", "placedAt", "orderDate", "date"} {
		if v, ok := order[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// orExtractOrderTotal searches common pricing fields in an order map and returns
// the largest positive value found, or nil when no numeric total is present.
func orExtractOrderTotal(order map[string]any) *float64 {
	var candidates []float64
	for _, key := range []string{"total", "totalValue", "amount", "grossValue", "orderValue", "finalPrice"} {
		orAddNumber(order[key], &candidates)
		if m, ok := order[key].(map[string]any); ok {
			orAddNumber(m["_total"], &candidates)
		}
	}
	for _, sectionKey := range []string{"pricing", "payment", "summary", "totals", "orderPricing"} {
		section, ok := order[sectionKey].(map[string]any)
		if !ok {
			continue
		}
		for _, valueKey := range []string{
			"totalPayment",
			"totalWithDeliveryCostAfterVoucherPayment",
			"totalWithDeliveryCost",
			"total",
		} {
			orAddNumber(section[valueKey], &candidates)
			if m, ok := section[valueKey].(map[string]any); ok {
				orAddNumber(m["_total"], &candidates)
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	var positives []float64
	for _, x := range candidates {
		if x > 0 {
			positives = append(positives, x)
		}
	}
	var best float64
	if len(positives) > 0 {
		best = positives[0]
		for _, x := range positives[1:] {
			if x > best {
				best = x
			}
		}
	} else {
		best = candidates[0]
		for _, x := range candidates[1:] {
			if x > best {
				best = x
			}
		}
	}
	return &best
}

// orAddNumber appends v to candidates if v is a numeric type.
func orAddNumber(v any, candidates *[]float64) {
	switch n := v.(type) {
	case float64:
		*candidates = append(*candidates, n)
	case int:
		*candidates = append(*candidates, float64(n))
	case int64:
		*candidates = append(*candidates, float64(n))
	}
}

// orTruthy returns true when v is a bool with value true.
func orTruthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

// orNonEmptyStr converts v to a trimmed string and reports whether it is non-empty.
func orNonEmptyStr(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s, s != ""
}

// orGetShippingAddressFromAccount fetches the user's saved shipping addresses and
// returns the preferred (default/current) one, falling back to the first entry.
func orGetShippingAddressFromAccount(s *session.Session, userID string) (map[string]any, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", userID)
	data, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	list, ok := data.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("no saved shipping addresses for user")
	}
	var preferred map[string]any
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if orTruthy(row["isDefault"]) || orTruthy(row["isCurrent"]) || orTruthy(row["isSelected"]) {
			preferred = row
			break
		}
	}
	chosen := preferred
	if chosen == nil {
		if row, ok := list[0].(map[string]any); ok {
			chosen = row
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no saved shipping addresses for user")
	}
	if sa, ok := chosen["shippingAddress"].(map[string]any); ok {
		return sa, nil
	}
	return chosen, nil
}

// orExtractDeliveryWindows recursively walks a calendar API response and collects
// unique delivery window objects (those with startsAt and endsAt), sorted by start time.
func orExtractDeliveryWindows(data any) []map[string]any {
	var windows []map[string]any
	var walk func(any)
	walk = func(obj any) {
		switch o := obj.(type) {
		case map[string]any:
			_, sok := orNonEmptyStr(o["startsAt"])
			_, eok := orNonEmptyStr(o["endsAt"])
			if sok && eok {
				windows = append(windows, map[string]any{
					"startsAt":       o["startsAt"],
					"endsAt":         o["endsAt"],
					"deliveryMethod": o["deliveryMethod"],
					"warehouse":      o["warehouse"],
					"closesAt":       o["closesAt"],
					"finalAt":        o["finalAt"],
				})
			}
			for _, v := range o {
				walk(v)
			}
		case []any:
			for _, v := range o {
				walk(v)
			}
		}
	}
	walk(data)
	uniq := make(map[string]map[string]any)
	for _, w := range windows {
		key := fmt.Sprintf("%v|%v|%v|%v", w["startsAt"], w["endsAt"], w["deliveryMethod"], w["warehouse"])
		uniq[key] = w
	}
	out := make([]map[string]any, 0, len(uniq))
	for _, w := range uniq {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["startsAt"]) < fmt.Sprint(out[j]["startsAt"])
	})
	return out
}

// orExtractReservableWindows is like orExtractDeliveryWindows but only returns windows
// that also have a deliveryMethod and warehouse (required for the reserve API call).
func orExtractReservableWindows(data any) []map[string]any {
	var windows []map[string]any
	var walk func(any)
	walk = func(obj any) {
		switch o := obj.(type) {
		case map[string]any:
			_, sok := orNonEmptyStr(o["startsAt"])
			_, eok := orNonEmptyStr(o["endsAt"])
			_, dmok := orNonEmptyStr(o["deliveryMethod"])
			_, whok := orNonEmptyStr(o["warehouse"])
			if sok && eok && dmok && whok {
				windows = append(windows, o)
			}
			for _, v := range o {
				walk(v)
			}
		case []any:
			for _, v := range o {
				walk(v)
			}
		}
	}
	walk(data)
	uniq := make(map[string]map[string]any)
	for _, w := range windows {
		key := fmt.Sprintf("%v|%v|%v|%v", w["startsAt"], w["endsAt"], w["deliveryMethod"], w["warehouse"])
		uniq[key] = w
	}
	out := make([]map[string]any, 0, len(uniq))
	for _, w := range uniq {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["startsAt"]) < fmt.Sprint(out[j]["startsAt"])
	})
	return out
}

// orDateAndHHMMFromISO splits an ISO 8601 timestamp into a date part (YYYY-MM-DD)
// and an HH:MM time part.
func orDateAndHHMMFromISO(ts string) (datePart, hhmm string) {
	idx := strings.IndexByte(ts, 'T')
	if idx < 0 {
		return ts, ""
	}
	end := idx + 6
	if end > len(ts) {
		end = len(ts)
	}
	return ts[:idx], ts[idx+1 : end]
}

// parseCalendarDate parses a YYYY-M-D or YYYY-MM-DD date string into year, month, day.
func parseCalendarDate(date string) (int, int, int, error) {
	parts := strings.Split(date, "-")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("date must be in format YYYY-M-D or YYYY-MM-DD")
	}
	y, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid year in date: %w", err)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid month in date: %w", err)
	}
	d, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid day in date: %w", err)
	}
	return y, m, d, nil
}
