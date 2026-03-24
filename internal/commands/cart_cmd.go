package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
	"github.com/rrudol/frisco/internal/tui"
)

func newCartCmd() *cobra.Command {
	var userID string
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Operacje na koszyku.",
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
	cmd.AddCommand(newCartShowCmd(), newCartAddCmd(), newCartRemoveCmd())
	return cmd
}

func newCartShowCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "show",
		Short: "Pobierz koszyk.",
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
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newCartAddCmd() *cobra.Command {
	var userID, productID string
	var quantity int
	c := &cobra.Command{
		Use:   "add",
		Short: "Dodaj/ustaw ilość produktu w koszyku.",
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
		Short: "Usuń produkt z koszyka (quantity=0).",
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
