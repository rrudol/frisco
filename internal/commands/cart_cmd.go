package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
	"github.com/rrudol/frisco/internal/tui"
)

func newCartCmd() *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "cart",
		Short: tr("Cart operations.", "Operacje na koszyku."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			return tui.RunCart(s, uid)
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "")
	cmd.AddCommand(newCartShowCmd(), newCartAddCmd(), newCartAddBatchCmd(), newCartRemoveCmd())
	return cmd
}

func newCartShowCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "show",
		Short: tr("Fetch cart.", "Pobierz koszyk."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			if err := printCartSummary(result); err == nil {
				return nil
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func printCartSummary(v any) error {
	root, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected cart payload")
	}
	rawProducts, ok := root["products"].([]any)
	if !ok {
		return fmt.Errorf("missing products list")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, tr("NAME\tPRODUCT ID\tQTY\tUNIT PRICE\tTOTAL", "NAZWA\tPRODUCT ID\tILOŚĆ\tCENA JEDN.\tWARTOŚĆ"))
	grandTotal := 0.0
	for _, raw := range rawProducts {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pid := asString(item["productId"])
		qty := asInt(item["quantity"])

		var product map[string]any
		if p, ok := item["product"].(map[string]any); ok {
			product = p
		}
		name := shared.ProductNameFromMap(product)
		if name == "" {
			name = pid
		}

		unitPrice := shared.MoneyString(item["price"])
		if unitPrice == "" && product != nil {
			unitPrice = shared.MoneyString(product["price"])
		}
		total := shared.MoneyString(item["total"])
		if total == "" {
			if p, ok := parseMoneyFloat(unitPrice); ok && qty > 0 {
				lineTotal := p * float64(qty)
				total = fmt.Sprintf("%.2f", lineTotal)
				grandTotal += lineTotal
			}
		} else if p, ok := parseMoneyFloat(total); ok {
			grandTotal += p
		}

		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%s\t%s\n",
			shared.TruncateText(name, 54),
			pid,
			qty,
			fallbackDash(unitPrice),
			fallbackDash(total),
		)
	}
	_ = w.Flush()

	if totalByStore, ok := root["total"].(map[string]any); ok {
		if val := shared.MoneyString(totalByStore["_total"]); val != "" {
			_, _ = fmt.Printf("\n%s %s\n", tr("Cart total:", "Suma koszyka:"), val)
		}
	} else if grandTotal > 0 {
		_, _ = fmt.Printf("\n%s %.2f\n", tr("Cart total:", "Suma koszyka:"), grandTotal)
	}
	return nil
}


func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}


func parseMoneyFloat(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" || s == "-" {
		return 0, false
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0, false
	}
	return f, true
}


func fallbackDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func newCartAddCmd() *cobra.Command {
	var userID, productID string
	var quantity int
	c := &cobra.Command{
		Use:   "add",
		Short: tr("Add/set product quantity in cart.", "Dodaj/ustaw ilość produktu w koszyku."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			body := map[string]any{
				"products": []any{
					map[string]any{"productId": productID, "quantity": quantity},
				},
			}
			result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", "")
	c.Flags().IntVar(&quantity, "quantity", 1, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("product-id")
	return c
}

func newCartRemoveCmd() *cobra.Command {
	var userID, productID string
	c := &cobra.Command{
		Use:   "remove",
		Short: tr("Remove product from cart (quantity=0).", "Usuń produkt z koszyka (quantity=0)."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
			body := map[string]any{
				"products": []any{
					map[string]any{"productId": productID, "quantity": 0},
				},
			}
			result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("product-id")
	return c
}
