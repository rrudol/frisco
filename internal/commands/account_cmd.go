package commands

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: tr("Account management operations.", "Operacje zarządzania kontem."),
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
		Short: tr("Fetch user profile.", "Pobierz profil użytkownika."),
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
		Short: tr("Shipping addresses.", "Adresy dostawy."),
	}
	cmd.AddCommand(newAccountAddressesListCmd(), newAccountAddressesAddCmd(), newAccountAddressesDeleteCmd())
	return cmd
}

func newAccountAddressesListCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "list",
		Short: tr("Address list.", "Lista adresów."),
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
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			return printAddressesTable(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func printAddressesTable(v any) error {
	list, ok := v.([]any)
	if !ok {
		return printJSON(v)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "id\trecipient\tstreet\tcity\tpostcode\tphone")
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := cellValue(row["id"])
		addr, _ := row["shippingAddress"].(map[string]any)
		recipient := cellValue(addr["recipient"])
		street := formatStreet(addr)
		city := cellValue(addr["city"])
		postcode := cellValue(addr["postcode"])
		phone := cellValue(addr["phoneNumber"])
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", id, recipient, street, city, postcode, phone)
	}
	return w.Flush()
}

func formatStreet(addr map[string]any) string {
	if addr == nil {
		return "—"
	}
	street := cellValue(addr["street"])
	building := cellValue(addr["buildingNumber"])
	apartment := cellValue(addr["apartmentNumber"])
	if street == "—" {
		return "—"
	}
	var sb strings.Builder
	sb.WriteString(street)
	if building != "—" {
		sb.WriteString(" ")
		sb.WriteString(building)
		if apartment != "—" {
			sb.WriteString("/")
			sb.WriteString(apartment)
		}
	}
	return sb.String()
}

func newAccountAddressesAddCmd() *cobra.Command {
	var userID, payloadFile string
	c := &cobra.Command{
		Use:   "add",
		Short: tr("Add address (JSON).", "Dodaj adres (JSON)."),
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
				return errors.New(tr("Payload file must contain a JSON object.", "Plik payload musi zawierać obiekt JSON."))
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
	c.Flags().StringVar(&payloadFile, "payload-file", "", tr("JSON address or {shippingAddress:{...}}", "JSON address lub {shippingAddress:{...}}"))
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("payload-file")
	return c
}

func newAccountAddressesDeleteCmd() *cobra.Command {
	var userID, addressID string
	c := &cobra.Command{
		Use:   "delete",
		Short: tr("Delete address by UUID.", "Usuń adres po UUID."),
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
		Short: tr("Consent management.", "Zarządzanie zgodami."),
	}
	cmd.AddCommand(newAccountConsentsShowCmd(), newAccountConsentsToggleCmd(), newAccountConsentsUpdateCmd())
	return cmd
}

func newAccountConsentsShowCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "show",
		Short: tr("Show current consents.", "Wyświetl aktualne zgody."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			consents, err := fetchConsents(s, uid)
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(consents)
			}
			return printConsentsTable(consents)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newAccountConsentsToggleCmd() *cobra.Command {
	var userID, key string
	var value bool
	c := &cobra.Command{
		Use:   "toggle",
		Short: tr("Toggle a single consent key.", "Przełącz pojedynczą zgodę."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			consents, err := fetchConsents(s, uid)
			if err != nil {
				return err
			}
			consents[key] = value
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/consents", uid)
			result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
				Data:       consents,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&key, "key", "", tr("Consent key to toggle (e.g. ad_storage).", "Klucz zgody do przełączenia (np. ad_storage)."))
	c.Flags().BoolVar(&value, "value", false, tr("Value to set (true/false).", "Wartość do ustawienia (true/false)."))
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("key")
	_ = c.MarkFlagRequired("value")
	return c
}

// fetchConsents retrieves the user profile and extracts preferences.consents.
func fetchConsents(s *session.Session, uid string) (map[string]any, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	profile, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New(tr("Unexpected profile response format.", "Nieoczekiwany format odpowiedzi profilu."))
	}
	prefs, _ := profile["preferences"].(map[string]any)
	if prefs == nil {
		return nil, errors.New(tr("No preferences found in profile.", "Brak preferencji w profilu."))
	}
	consents, _ := prefs["consents"].(map[string]any)
	if consents == nil {
		return nil, errors.New(tr("No consents found in profile.", "Brak zgód w profilu."))
	}
	return consents, nil
}

func printConsentsTable(consents map[string]any) error {
	keys := make([]string, 0, len(consents))
	for k := range consents {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "key\tenabled")
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s\t%v\n", k, consents[k])
	}
	return w.Flush()
}

func newAccountConsentsUpdateCmd() *cobra.Command {
	var userID, payloadFile string
	c := &cobra.Command{
		Use:   "update",
		Short: tr("Update consents using JSON payload.", "Aktualizuj zgody payloadem JSON."),
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
				return errors.New(tr("Payload file must contain a JSON object.", "Plik payload musi zawierać obiekt JSON."))
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
		Short: tr("Rules acceptance.", "Akceptacja regulaminów."),
	}
	cmd.AddCommand(newAccountRulesAcceptCmd())
	return cmd
}

func newAccountRulesAcceptCmd() *cobra.Command {
	var userID, payloadFile string
	var ruleIDs []string
	c := &cobra.Command{
		Use:   "accept",
		Short: tr("Accept rules.", "Akceptuj reguły."),
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
					return errors.New(tr("Payload file must contain a JSON object.", "Plik payload musi zawierać obiekt JSON."))
				}
			} else {
				if len(ruleIDs) == 0 {
					return errors.New(tr("Provide --rule-id or --payload-file.", "Podaj --rule-id albo --payload-file."))
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
	c.Flags().StringArrayVar(&ruleIDs, "rule-id", nil, tr("Repeatable rule UUIDs to accept.", "Powtarzalne UUID reguł do akceptacji."))
	c.Flags().StringVar(&payloadFile, "payload-file", "", tr("Alternative: full JSON payload.", "Alternatywnie pełny payload JSON."))
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newAccountVouchersCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "vouchers",
		Short: tr("Fetch vouchers.", "Pobierz vouchery."),
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
	var pageIndex, pageSize int
	c := &cobra.Command{
		Use:   "payments",
		Short: tr("Fetch payment methods.", "Pobierz metody płatności."),
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
			q := []string{
				fmt.Sprintf("pageIndex=%d", pageIndex),
				fmt.Sprintf("pageSize=%d", pageSize),
			}
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			return printPaymentsTable(result)
		},
	}
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 25, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func printPaymentsTable(v any) error {
	page, ok := v.(map[string]any)
	if !ok {
		return printJSON(v)
	}

	// Pagination info.
	pageIndex := int(toFloat(page["pageIndex"]))
	pageCount := int(toFloat(page["pageCount"]))
	totalCount := int(toFloat(page["totalCount"]))
	fmt.Printf("Page %d/%d (%d total)\n\n", pageIndex, pageCount, totalCount)

	items := toSlice(page["items"])
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "date\tstatus\tchannel\tcard\torderId")
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		date := cellValue(row["createdAt"])
		if len(date) >= 10 {
			date = date[:10]
		}
		status := cellValue(row["status"])
		channel := cellValue(row["channelName"])
		card := cellValue(row["creditCardBrand"])
		orderID := cellValue(row["orderId"])
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", date, status, channel, card, orderID)
	}
	return w.Flush()
}

func newAccountMembershipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "membership",
		Short: tr("Membership cards/points.", "Karty i punkty membership."),
	}
	cmd.AddCommand(newAccountMembershipCardsCmd(), newAccountMembershipPointsCmd())
	return cmd
}

func newAccountMembershipCardsCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "cards",
		Short: tr("Fetch membership cards.", "Pobierz membership cards."),
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
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			if list, ok := result.([]any); ok && len(list) == 0 {
				fmt.Println(tr("No membership cards.", "Brak kart membership."))
				return nil
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newAccountMembershipPointsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "points",
		Short: tr("Membership points.", "Punkty membership."),
	}
	cmd.AddCommand(newAccountMembershipPointsShowCmd(), newAccountMembershipPointsHistoryCmd())
	return cmd
}

func newAccountMembershipPointsShowCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "show",
		Short: tr("Show points summary (balance, earned, spent).", "Podsumowanie punktów (saldo, zdobyte, wydane)."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			basePath := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership/points", uid)

			// Fetch first page to learn pageCount.
			q := []string{"pageIndex=1", "pageSize=100"}
			first, err := httpclient.RequestJSON(s, "GET", basePath, httpclient.RequestOpts{Query: q})
			if err != nil {
				return err
			}
			page, ok := first.(map[string]any)
			if !ok {
				return errors.New(tr("Unexpected response format.", "Nieoczekiwany format odpowiedzi."))
			}
			pageCount := int(toFloat(page["pageCount"]))
			allItems := toSlice(page["items"])

			// Fetch remaining pages.
			for pi := 2; pi <= pageCount; pi++ {
				q := []string{
					fmt.Sprintf("pageIndex=%d", pi),
					"pageSize=100",
				}
				res, err := httpclient.RequestJSON(s, "GET", basePath, httpclient.RequestOpts{Query: q})
				if err != nil {
					return err
				}
				if p, ok := res.(map[string]any); ok {
					allItems = append(allItems, toSlice(p["items"])...)
				}
			}

			// Compute summary.
			var balance, earned, spent float64
			for _, item := range allItems {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				pts := toFloat(row["membershipPoints"])
				balance += pts
				if pts > 0 {
					earned += pts
				} else {
					spent += math.Abs(pts)
				}
			}

			if strings.EqualFold(outputFormat, "json") {
				summary := map[string]any{
					"balance":      int(balance),
					"earned":       int(earned),
					"spent":        int(spent),
					"transactions": len(allItems),
				}
				return printJSON(summary)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "balance\t%d\n", int(balance))
			_, _ = fmt.Fprintf(w, "earned\t%d\n", int(earned))
			_, _ = fmt.Fprintf(w, "spent\t%d\n", int(spent))
			_, _ = fmt.Fprintf(w, "transactions\t%d\n", len(allItems))
			return w.Flush()
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newAccountMembershipPointsHistoryCmd() *cobra.Command {
	var userID string
	var pageIndex, pageSize int
	c := &cobra.Command{
		Use:   "history",
		Short: tr("Show points history (paginated).", "Historia punktów (stronicowana)."),
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
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			return printPointsHistoryTable(result)
		},
	}
	c.Flags().IntVar(&pageIndex, "page-index", 1, "")
	c.Flags().IntVar(&pageSize, "page-size", 25, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func printPointsHistoryTable(v any) error {
	page, ok := v.(map[string]any)
	if !ok {
		return printJSON(v)
	}
	items := toSlice(page["items"])
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "date\toperation\tpoints\torderId")
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		date := cellValue(row["createdAt"])
		if len(date) >= 10 {
			date = date[:10]
		}
		operation := cellValue(row["operation"])
		points := cellValue(row["membershipPoints"])
		orderID := cellValue(row["orderId"])
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", date, operation, points, orderID)
	}
	return w.Flush()
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func toSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}
