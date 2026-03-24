package commands

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
)

func newOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Order details.",
	}
	cmd.AddCommand(
		newOrdersListCmd(),
		newOrdersGetCmd(),
		newOrdersDeliveryCmd(),
		newOrdersPaymentsCmd(),
		newOrdersProductsCmd(),
	)
	return cmd
}

// orderProduct is a parsed product line from an order.
type orderProduct struct {
	ProductID string
	Name      string
	Quantity  float64
	Price     float64
	Total     float64
	Grammage  float64
	Unit      string
}

// extractOrderProducts parses the "products" array from an order map.
func extractOrderProducts(order map[string]any) []orderProduct {
	raw, ok := order["products"].([]any)
	if !ok {
		return nil
	}
	var out []orderProduct
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		op := orderProduct{}
		if v, ok := m["productId"].(string); ok {
			op.ProductID = v
		}
		if v, ok := m["quantity"].(float64); ok {
			op.Quantity = v
		}
		if v, ok := m["price"].(float64); ok {
			op.Price = v
		}
		if v, ok := m["total"].(float64); ok {
			op.Total = v
		}
		// Extract name and product-level fields from nested "product" map.
		if prod, ok := m["product"].(map[string]any); ok {
			op.Name = shared.LocalizedString(prod["name"])
			if v, ok := prod["grammage"].(float64); ok {
				op.Grammage = v
			}
			if v, ok := prod["unitOfMeasure"].(string); ok {
				op.Unit = v
			}
			if op.Name == "" {
				if v, ok := prod["brand"].(string); ok {
					op.Name = v
				}
			}
		}
		// Fallback: name at the top level of the product entry.
		if op.Name == "" {
			op.Name = shared.LocalizedString(m["name"])
		}
		out = append(out, op)
	}
	return out
}

// printOrderProductsTable renders products as a tabwriter table and prints a
// summary line. sortBy may be "total", "name", or "" (API order).
func printOrderProductsTable(products []orderProduct, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "total":
		sort.Slice(products, func(i, j int) bool {
			return products[i].Total > products[j].Total
		})
	case "name":
		sort.Slice(products, func(i, j int) bool {
			return strings.ToLower(products[i].Name) < strings.ToLower(products[j].Name)
		})
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tQTY\tPRICE\tTOTAL\tGRAMMAGE\tUNIT")
	for _, p := range products {
		qty := fmt.Sprintf("%.0f", p.Quantity)
		price := fmt.Sprintf("%.2f", p.Price)
		total := fmt.Sprintf("%.2f", p.Total)
		grammage := ""
		if p.Grammage != 0 {
			grammage = fmt.Sprintf("%g", p.Grammage)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			p.Name, qty, price, total, grammage, p.Unit)
	}
	_ = w.Flush()

	var orderTotal float64
	for _, p := range products {
		orderTotal += p.Total
	}
	fmt.Printf("\nOrder total: %.2f PLN (%d items)\n",
		math.Round(orderTotal*100)/100, len(products))
}

// extractOrdersList extracts an orders slice from various API response shapes.
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

// extractOrderDatetime returns the first non-empty date/time string found in an order map.
func extractOrderDatetime(order map[string]any) string {
	for _, key := range []string{"createdAt", "created", "placedAt", "orderDate", "date"} {
		if v, ok := order[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extractOrderTotal searches common pricing fields in an order map and returns the
// largest positive value found, or nil when no numeric total is present.
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

// addNumber appends v to candidates if v is a numeric type.
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
		Short: "Order list.",
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
			type orderRow struct {
				date, status, total, orderID string
				totalVal                     *float64
			}
			var rows []orderRow
			for _, order := range items {
				id := order["id"]
				if id == nil {
					id = order["orderId"]
				}
				st := order["status"]
				if st == nil {
					st = order["orderStatus"]
				}
				createdAt := extractOrderDatetime(order)
				date := createdAt
				if len(date) >= 10 {
					date = date[:10]
				}
				totalPtr := extractOrderTotal(order)
				totalStr := "—"
				if totalPtr != nil {
					totalStr = fmt.Sprintf("%.2f", math.Round(*totalPtr*100)/100)
				}
				rows = append(rows, orderRow{
					date:    date,
					status:  fmt.Sprint(st),
					total:   totalStr,
					orderID: fmt.Sprint(id),
					totalVal: totalPtr,
				})
			}
			if strings.EqualFold(outputFormat, "json") {
				var compact []map[string]any
				for _, r := range rows {
					row := map[string]any{
						"id":        r.orderID,
						"status":    r.status,
						"createdAt": r.date,
					}
					if r.totalVal != nil {
						row["totalPLN"] = math.Round(*r.totalVal*100) / 100
					} else {
						row["totalPLN"] = nil
					}
					compact = append(compact, row)
				}
				var totalVals []float64
				for _, r := range rows {
					if r.totalVal != nil {
						totalVals = append(totalVals, *r.totalVal)
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
			}
			// Table output.
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "date\tstatus\ttotal\torderId")
			for _, r := range rows {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.date, r.status, r.total, r.orderID)
			}
			_ = w.Flush()
			// Summary line.
			var totalVals []float64
			for _, r := range rows {
				if r.totalVal != nil {
					totalVals = append(totalVals, *r.totalVal)
				}
			}
			if len(totalVals) > 0 {
				var sum float64
				for _, v := range totalVals {
					sum += v
				}
				avg := math.Round(sum/float64(len(totalVals))*100) / 100
				sum = math.Round(sum*100) / 100
				fmt.Printf("\n%d orders | total: %.2f PLN | avg: %.2f PLN\n", len(rows), sum, avg)
			} else {
				fmt.Printf("\n%d orders\n", len(rows))
			}
			return nil
		},
	}
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 10, "")
	c.Flags().BoolVar(&allPages, "all-pages", false, "Fetch all pages.")
	c.Flags().BoolVar(&rawOut, "raw", false, "Return raw API response.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newOrdersGetCmd() *cobra.Command {
	var userID, orderID string
	c := &cobra.Command{
		Use:   "get",
		Short: "Single order details.",
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
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			order, ok := result.(map[string]any)
			if !ok {
				return printJSON(result)
			}
			// Print order summary header.
			id := order["id"]
			if id == nil {
				id = order["orderId"]
			}
			status := order["status"]
			if status == nil {
				status = order["orderStatus"]
			}
			date := extractOrderDatetime(order)
			if len(date) >= 10 {
				date = date[:10]
			}
			totalPtr := extractOrderTotal(order)
			totalStr := "—"
			if totalPtr != nil {
				totalStr = fmt.Sprintf("%.2f PLN", math.Round(*totalPtr*100)/100)
			}
			products := extractOrderProducts(order)
			fmt.Printf("Order ID : %v\n", id)
			fmt.Printf("Status   : %v\n", status)
			fmt.Printf("Date     : %s\n", date)
			fmt.Printf("Total    : %s\n", totalStr)
			fmt.Printf("Products : %d items\n\n", len(products))
			if len(products) > 0 {
				printOrderProductsTable(products, "")
			}
			return nil
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
		Short: "Delivery details for order.",
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
		Short: "Payments for order.",
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

func newOrdersProductsCmd() *cobra.Command {
	var userID, orderID, sortBy string
	c := &cobra.Command{
		Use:   "products",
		Short: "List products in an order.",
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
			order, ok := result.(map[string]any)
			if !ok {
				return fmt.Errorf("unexpected response shape")
			}
			products := extractOrderProducts(order)

			if strings.EqualFold(outputFormat, "json") {
				var out []map[string]any
				for _, p := range products {
					out = append(out, map[string]any{
						"product_id": p.ProductID,
						"name":       p.Name,
						"quantity":   p.Quantity,
						"price":      math.Round(p.Price*100) / 100,
						"total":      math.Round(p.Total*100) / 100,
						"grammage":   p.Grammage,
						"unit":       p.Unit,
					})
				}
				return printJSON(out)
			}

			if len(products) == 0 {
				fmt.Println("No products found in order.")
				return nil
			}
			printOrderProductsTable(products, sortBy)
			return nil
		},
	}
	c.Flags().StringVar(&orderID, "order-id", "", "Order ID (required)")
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().StringVar(&sortBy, "sort-by", "", "Sort by: total or name (default: API order)")
	_ = c.MarkFlagRequired("order-id")
	return c
}
