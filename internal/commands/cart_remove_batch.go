package commands

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newCartRemoveBatchCmd() *cobra.Command {
	var userID, productIDs string
	c := &cobra.Command{
		Use:   "remove-batch",
		Short: "Remove multiple products from cart by setting quantity=0.",
		Long:  "Loads current cart (GET), sets quantity=0 for each listed product ID, then PUTs the updated cart back.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(productIDs) == "" {
				return fmt.Errorf("--product-ids is required")
			}

			rawIDs := strings.Split(productIDs, ",")
			toRemove := make(map[string]bool)
			for _, id := range rawIDs {
				id = strings.TrimSpace(id)
				if id != "" {
					toRemove[id] = true
				}
			}
			if len(toRemove) == 0 {
				return fmt.Errorf("no valid product IDs provided")
			}

			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)

			current, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}

			qtyMap := quantitiesFromCartGET(current)

			// Track which requested IDs were actually in the cart.
			removed := []string{}
			notFound := []string{}
			for id := range toRemove {
				if _, inCart := qtyMap[id]; inCart {
					delete(qtyMap, id)
					removed = append(removed, id)
				} else {
					notFound = append(notFound, id)
				}
			}

			// Build the products slice that keeps remaining items (quantity > 0).
			products := mergedCartProductsSlice(qtyMap)

			body := map[string]any{"products": products}
			last, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}

			if strings.EqualFold(outputFormat, "json") {
				return printJSON(map[string]any{
					"removed":           removed,
					"notFoundInCart":    notFound,
					"remainingProducts": len(products),
					"putCart":           last,
				})
			}

			// Human-readable summary.
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if len(removed) > 0 {
				_, _ = fmt.Fprintln(w, "REMOVED\tPRODUCT ID")
				for _, id := range removed {
					_, _ = fmt.Fprintf(w, "yes\t%s\n", id)
				}
			}
			if len(notFound) > 0 {
				_, _ = fmt.Fprintln(w, "NOT IN CART\tPRODUCT ID")
				for _, id := range notFound {
					_, _ = fmt.Fprintf(w, "skipped\t%s\n", id)
				}
			}
			_ = w.Flush()

			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"\nRemoved %d product(s). Cart now has %d line(s).\n",
				len(removed),
				len(products),
			)
			return nil
		},
	}
	c.Flags().StringVar(&productIDs, "product-ids", "", "Comma-separated list of product IDs to remove.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("product-ids")
	return c
}
