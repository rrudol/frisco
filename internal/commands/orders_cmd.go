package commands

import (
	"fmt"
	"math"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Szczegóły zamówień.",
	}
	cmd.AddCommand(
		newOrdersListCmd(),
		newOrdersGetCmd(),
		newOrdersDeliveryCmd(),
		newOrdersPaymentsCmd(),
	)
	return cmd
}

func extractOrdersList(payload any) []map[string]any {
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

func extractOrderDatetime(order map[string]any) string {
	for _, key := range []string{"createdAt", "created", "placedAt", "orderDate", "date"} {
		if v, ok := order[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func extractOrderTotal(order map[string]any) *float64 {
	var candidates []float64
	for _, key := range []string{"total", "totalValue", "amount", "grossValue", "orderValue", "finalPrice"} {
		addNumber(order[key], &candidates)
		if m, ok := order[key].(map[string]any); ok {
			addNumber(m["_total"], &candidates)
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
			addNumber(section[valueKey], &candidates)
			if m, ok := section[valueKey].(map[string]any); ok {
				addNumber(m["_total"], &candidates)
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

func addNumber(v any, candidates *[]float64) {
	switch n := v.(type) {
	case float64:
		*candidates = append(*candidates, n)
	case int:
		*candidates = append(*candidates, float64(n))
	case int64:
		*candidates = append(*candidates, float64(n))
	}
}

func newOrdersListCmd() *cobra.Command {
	var (
		userID              string
		pageIndex, pageSize int
		allPages, rawOut    bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "Lista zamówień.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders", uid)
			var result any
			if allPages {
				var allItems []map[string]any
				pi := pageIndex
				for {
					q := []string{
						fmt.Sprintf("pageIndex=%d", pi),
						fmt.Sprintf("pageSize=%d", pageSize),
					}
					payload, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
					if err != nil {
						return err
					}
					items := extractOrdersList(payload)
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
					return err
				}
			}
			if rawOut {
				return printJSON(result)
			}
			items := extractOrdersList(result)
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
					"createdAt": extractOrderDatetime(order),
				}
				if t := extractOrderTotal(order); t != nil {
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
			return printJSON(map[string]any{"summary": summary, "orders": compact})
		},
	}
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 10, "")
	c.Flags().BoolVar(&allPages, "all-pages", false, "Pobierz wszystkie strony.")
	c.Flags().BoolVar(&rawOut, "raw", false, "Zwróć surową odpowiedź API.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newOrdersGetCmd() *cobra.Command {
	var userID, orderID string
	c := &cobra.Command{
		Use:   "get",
		Short: "Szczegóły jednego zamówienia.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s", uid, orderID)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&orderID, "order-id", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("order-id")
	return c
}

func newOrdersDeliveryCmd() *cobra.Command {
	var userID, orderID string
	c := &cobra.Command{
		Use:   "delivery",
		Short: "Dostawa dla zamówienia.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/delivery", uid, orderID)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&orderID, "order-id", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("order-id")
	return c
}

func newOrdersPaymentsCmd() *cobra.Command {
	var userID, orderID string
	c := &cobra.Command{
		Use:   "payments",
		Short: "Płatności dla zamówienia.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/payments", uid, orderID)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&orderID, "order-id", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("order-id")
	return c
}
