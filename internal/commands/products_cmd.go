package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/shared"
)

func newProductsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "products",
		Short: tr("Product operations.", "Operacje produktowe."),
	}
	cmd.AddCommand(newProductsSearchCmd(), newProductsByIDsCmd(), newProductsNutritionCmd())
	return cmd
}

func newProductsSearchCmd() *cobra.Command {
	var (
		search, deliveryMethod, userID, categoryID string
		pageIndex, pageSize                        int
	)
	c := &cobra.Command{
		Use:   "search",
		Short: tr("Search products.", "Szukaj produktów."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products/query", uid)
			q := []string{
				"purpose=Listing",
				fmt.Sprintf("pageIndex=%d", pageIndex),
				fmt.Sprintf("search=%s", search),
				"includeFacets=true",
				fmt.Sprintf("deliveryMethod=%s", deliveryMethod),
				fmt.Sprintf("pageSize=%d", pageSize),
				"language=pl",
				"disableAutocorrect=false",
			}
			if strings.TrimSpace(categoryID) != "" {
				q = append(q, fmt.Sprintf("categoryId=%s", strings.TrimSpace(categoryID)))
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&search, "search", "", tr("Search phrase.", "Fraza wyszukiwania."))
	c.Flags().StringVar(&categoryID, "category-id", "", tr("Frisco categoryId (narrows listing, e.g. 18703 Warzywa i owoce).", "Frisco categoryId (zawęża listę, np. 18703 Warzywa i owoce)."))
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 84, "")
	c.Flags().StringVar(&deliveryMethod, "delivery-method", "Van", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("search")
	return c
}

func newProductsByIDsCmd() *cobra.Command {
	var userID string
	var productIDs []string
	c := &cobra.Command{
		Use:   "by-ids",
		Short: tr("Fetch products by productIds.", "Pobierz produkty po productIds."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products", uid)
			var q []string
			for _, pid := range productIDs {
				q = append(q, fmt.Sprintf("productIds=%s", pid))
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringArrayVar(&productIDs, "product-id", nil, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("product-id")
	return c
}

func newProductsNutritionCmd() *cobra.Command {
	var productID string
	var rawOutput bool
	c := &cobra.Command{
		Use:   "nutrition",
		Short: tr("Fetch product nutrition values (if available).", "Pobierz wartości odżywcze produktu (jeśli dostępne)."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/content/api/v1/products/get/%s", productID)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			if rawOutput {
				return printJSON(result)
			}

			nutrition := shared.ExtractNutritionBlock(result)
			if nutrition == nil {
				return printJSON(map[string]any{
					"productId": productID,
					"message": tr(
						"No explicit nutrition values found in this endpoint. Use --raw to inspect full response.",
						"Brak jawnych wartości odżywczych w tym endpointcie. Użyj --raw, żeby zobaczyć pełną odpowiedź.",
					),
				})
			}
			return printJSON(map[string]any{
				"productId":  productID,
				"nutrition":  nutrition,
				"sourcePath": "/app/content/api/v1/products/get/{id}",
			})
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", tr("Product ID", "ID produktu"))
	c.Flags().BoolVar(&rawOutput, "raw", false, tr("Show full API response", "Pokaż pełną odpowiedź API"))
	_ = c.MarkFlagRequired("product-id")
	return c
}

