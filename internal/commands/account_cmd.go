package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Operacje zarządzania kontem.",
	}
	cmd.AddCommand(
		newAccountProfileCmd(),
		newAccountAddressesCmd(),
		newAccountConsentsCmd(),
		newAccountRulesCmd(),
		newAccountVouchersCmd(),
		newAccountPaymentsCmd(),
		newAccountMembershipCmd(),
	)
	return cmd
}

func newAccountProfileCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "profile",
		Short: "Pobierz profil użytkownika.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s", uid)
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

func newAccountAddressesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addresses",
		Short: "Adresy dostawy.",
	}
	cmd.AddCommand(newAccountAddressesListCmd(), newAccountAddressesAddCmd(), newAccountAddressesDeleteCmd())
	return cmd
}

func newAccountAddressesListCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "list",
		Short: "Lista adresów.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", uid)
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

func newAccountAddressesAddCmd() *cobra.Command {
	var userID, payloadFile string
	c := &cobra.Command{
		Use:   "add",
		Short: "Dodaj adres (JSON).",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			raw, err := loadJSONFile(payloadFile)
			if err != nil {
				return err
			}
			data, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("Plik payload musi zawierać obiekt JSON.")
			}
			var body map[string]any
			if _, has := data["shippingAddress"]; has {
				body = data
			} else {
				body = map[string]any{"shippingAddress": data}
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", uid)
			result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
				Data:       body,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&payloadFile, "payload-file", "", "JSON address lub {shippingAddress:{...}}")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("payload-file")
	return c
}

func newAccountAddressesDeleteCmd() *cobra.Command {
	var userID, addressID string
	c := &cobra.Command{
		Use:   "delete",
		Short: "Usuń adres po UUID.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses/%s", uid, addressID)
			result, err := httpclient.RequestJSON(s, "DELETE", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&addressID, "address-id", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("address-id")
	return c
}

func newAccountConsentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consents",
		Short: "Zarządzanie zgodami.",
	}
	cmd.AddCommand(newAccountConsentsUpdateCmd())
	return cmd
}

func newAccountConsentsUpdateCmd() *cobra.Command {
	var userID, payloadFile string
	c := &cobra.Command{
		Use:   "update",
		Short: "Aktualizuj zgody payloadem JSON.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			raw, err := loadJSONFile(payloadFile)
			if err != nil {
				return err
			}
			body, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("Plik payload musi zawierać obiekt JSON.")
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/consents", uid)
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
	c.Flags().StringVar(&payloadFile, "payload-file", "", "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("payload-file")
	return c
}

func newAccountRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Akceptacja regulaminów.",
	}
	cmd.AddCommand(newAccountRulesAcceptCmd())
	return cmd
}

func newAccountRulesAcceptCmd() *cobra.Command {
	var userID, payloadFile string
	var ruleIDs []string
	c := &cobra.Command{
		Use:   "accept",
		Short: "Akceptuj reguły.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			var body map[string]any
			if payloadFile != "" {
				raw, err := loadJSONFile(payloadFile)
				if err != nil {
					return err
				}
				var ok bool
				body, ok = raw.(map[string]any)
				if !ok {
					return fmt.Errorf("Plik payload musi zawierać obiekt JSON.")
				}
			} else {
				if len(ruleIDs) == 0 {
					return fmt.Errorf("Podaj --rule-id albo --payload-file.")
				}
				body = map[string]any{"acceptedRules": ruleIDs}
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/rules", uid)
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
	c.Flags().StringArrayVar(&ruleIDs, "rule-id", nil, "Powtarzalne UUID reguł do akceptacji.")
	c.Flags().StringVar(&payloadFile, "payload-file", "", "Alternatywnie pełny payload JSON.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newAccountVouchersCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "vouchers",
		Short: "Pobierz vouchery.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/vouchers", uid)
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

func newAccountPaymentsCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "payments",
		Short: "Pobierz metody płatności.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/payments", uid)
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

func newAccountMembershipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "membership",
		Short: "Membership cards/points.",
	}
	cmd.AddCommand(newAccountMembershipCardsCmd(), newAccountMembershipPointsCmd())
	return cmd
}

func newAccountMembershipCardsCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "cards",
		Short: "Pobierz membership cards.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership-cards", uid)
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

func newAccountMembershipPointsCmd() *cobra.Command {
	var userID string
	var pageIndex, pageSize int
	c := &cobra.Command{
		Use:   "points",
		Short: "Pobierz historię punktów.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership/points", uid)
			q := []string{
				fmt.Sprintf("pageIndex=%d", pageIndex),
				fmt.Sprintf("pageSize=%d", pageSize),
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 25, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}
