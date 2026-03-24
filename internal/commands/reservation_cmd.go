package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newReservationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservation",
		Short: tr("Reservation planning and status.", "Planowanie i status rezerwacji."),
	}
	cmd.AddCommand(
		newReservationDeliveryOptionsCmd(),
		newReservationCalendarCmd(),
		newReservationSlotsCmd(),
		newReservationReserveCmd(),
		newReservationPlanCmd(),
		newReservationStatusCmd(),
		newReservationCancelCmd(),
	)
	return cmd
}

func newReservationDeliveryOptionsCmd() *cobra.Command {
	var postcode string
	c := &cobra.Command{
		Use:   "delivery-options",
		Short: tr("Delivery/payment options by postcode.", "Opcje dostawy/płatności po kodzie pocztowym."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			path := "/app/commerce/api/v1/calendar/delivery-payment"
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{
				Query: []string{"postcode=" + postcode},
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&postcode, "postcode", "", "")
	_ = c.MarkFlagRequired("postcode")
	return c
}

func newReservationCalendarCmd() *cobra.Command {
	var (
		shippingFile, date, userID string
	)
	c := &cobra.Command{
		Use:   "calendar",
		Short: tr("Available delivery windows for address.", "Dostępne okna czasowe dla adresu."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			raw, err := loadJSONFile(shippingFile)
			if err != nil {
				return err
			}
			shippingAddress, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf(tr("Address file must contain a JSON object.", "Plik z adresem musi zawierać obiekt JSON."))
			}
			body := map[string]any{"shippingAddress": shippingAddress}
			var path string
			if date != "" {
				parts := strings.Split(date, "-")
				if len(parts) != 3 {
					return fmt.Errorf(tr("Date must be in format YYYY-M-D or YYYY-MM-DD", "Data musi mieć format YYYY-M-D lub YYYY-MM-DD"))
				}
				y, err1 := strconv.Atoi(parts[0])
				m, err2 := strconv.Atoi(parts[1])
				d, err3 := strconv.Atoi(parts[2])
				if err1 != nil || err2 != nil || err3 != nil {
					return fmt.Errorf(tr("Date must be in format YYYY-M-D or YYYY-MM-DD", "Data musi mieć format YYYY-M-D lub YYYY-MM-DD"))
				}
				path = fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d", uid, y, m, d)
			} else {
				path = fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van", uid)
			}
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
	c.Flags().StringVar(&shippingFile, "shipping-address-file", "", tr("JSON with shippingAddress.", "JSON z shippingAddress."))
	c.Flags().StringVar(&date, "date", "", tr("Optional date YYYY-M-D", "Opcjonalnie data YYYY-M-D"))
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("shipping-address-file")
	return c
}

func getShippingAddressFromAccount(s *session.Session, userID string) (map[string]any, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", userID)
	data, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	list, ok := data.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf(tr("No saved user addresses.", "Brak zapisanych adresów użytkownika."))
	}
	var preferred map[string]any
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if truthy(row["isDefault"]) || truthy(row["isCurrent"]) || truthy(row["isSelected"]) {
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
		return nil, fmt.Errorf(tr("No saved user addresses.", "Brak zapisanych adresów użytkownika."))
	}
	if sa, ok := chosen["shippingAddress"].(map[string]any); ok {
		return sa, nil
	}
	return chosen, nil
}

func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func nonEmptyStr(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s, s != ""
}

func extractDeliveryWindows(data any) []map[string]any {
	var windows []map[string]any
	var walk func(any)
	walk = func(obj any) {
		switch o := obj.(type) {
		case map[string]any:
			_, sok := nonEmptyStr(o["startsAt"])
			_, eok := nonEmptyStr(o["endsAt"])
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

func extractReservableWindows(data any) []map[string]any {
	var windows []map[string]any
	var walk func(any)
	walk = func(obj any) {
		switch o := obj.(type) {
		case map[string]any:
			_, sok := nonEmptyStr(o["startsAt"])
			_, eok := nonEmptyStr(o["endsAt"])
			_, dmok := nonEmptyStr(o["deliveryMethod"])
			_, whok := nonEmptyStr(o["warehouse"])
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

func dateAndHHMMFromISO(ts string) (datePart, hhmm string) {
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

func newReservationSlotsCmd() *cobra.Command {
	var (
		days         int
		startDate    string
		shippingFile string
		userID       string
		rawOut       bool
	)
	c := &cobra.Command{
		Use: "slots",
		Short: tr(
			"Get available delivery slots for upcoming days (including today).",
			"Pobierz dostępne godziny dostawy dla kolejnych dni (w tym dzisiaj).",
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			var shippingAddress map[string]any
			if shippingFile != "" {
				raw, err := loadJSONFile(shippingFile)
				if err != nil {
					return err
				}
				var ok bool
				shippingAddress, ok = raw.(map[string]any)
				if !ok {
					return fmt.Errorf(tr("Address file must contain a JSON object.", "Plik z adresem musi zawierać obiekt JSON."))
				}
			} else {
				shippingAddress, err = getShippingAddressFromAccount(s, uid)
				if err != nil {
					return err
				}
			}
			var baseDate time.Time
			if startDate != "" {
				baseDate, err = time.Parse("2006-01-02", startDate)
				if err != nil {
					return err
				}
			} else {
				baseDate = time.Now().Truncate(24 * time.Hour)
			}
			allDays := map[string]any{}
			var pretty []map[string]any
			for i := 0; i < days; i++ {
				d := baseDate.AddDate(0, 0, i)
				path := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d",
					uid, d.Year(), int(d.Month()), d.Day())
				dayData, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
					Data:       map[string]any{"shippingAddress": shippingAddress},
					DataFormat: httpclient.FormatJSON,
				})
				if err != nil {
					return err
				}
				dayKey := d.Format("2006-01-02")
				allDays[dayKey] = dayData
				pretty = append(pretty, map[string]any{
					"date":  dayKey,
					"slots": extractDeliveryWindows(dayData),
				})
			}
			if rawOut {
				return printJSON(allDays)
			}
			return printJSON(map[string]any{"days": pretty})
		},
	}
	c.Flags().IntVar(&days, "days", 3, tr("How many upcoming days to check.", "Ile kolejnych dni sprawdzić."))
	c.Flags().StringVar(&startDate, "start-date", "", tr("Start date YYYY-MM-DD (default: today).", "Data startowa YYYY-MM-DD (domyślnie dziś)."))
	c.Flags().StringVar(&shippingFile, "shipping-address-file", "", tr("Optional address JSON.", "Opcjonalny JSON z adresem."))
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().BoolVar(&rawOut, "raw", false, tr("Return raw API response.", "Zwróć surową odpowiedź API."))
	return c
}

func newReservationReserveCmd() *cobra.Command {
	var (
		date, fromTime, toTime, shippingFile, userID string
	)
	c := &cobra.Command{
		Use:   "reserve",
		Short: tr("Reserve slot by date and times (e.g. 06:00-07:00).", "Zarezerwuj slot po dacie i godzinach (np. 06:00-07:00)."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			targetDate, err := time.Parse("2006-01-02", date)
			if err != nil {
				return err
			}
			var shippingAddress map[string]any
			if shippingFile != "" {
				raw, err := loadJSONFile(shippingFile)
				if err != nil {
					return err
				}
				var ok bool
				shippingAddress, ok = raw.(map[string]any)
				if !ok {
					return fmt.Errorf(tr("Address file must contain a JSON object.", "Plik z adresem musi zawierać obiekt JSON."))
				}
			} else {
				shippingAddress, err = getShippingAddressFromAccount(s, uid)
				if err != nil {
					return err
				}
			}
			calPath := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d",
				uid, targetDate.Year(), int(targetDate.Month()), targetDate.Day())
			dayData, err := httpclient.RequestJSON(s, "POST", calPath, httpclient.RequestOpts{
				Data:       map[string]any{"shippingAddress": shippingAddress},
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			windows := extractReservableWindows(dayData)
			if len(windows) == 0 {
				return fmt.Errorf(tr("No reservable slots for given date.", "Brak dostępnych slotów rezerwacji dla podanej daty."))
			}
			var selected map[string]any
			var possible []string
			for _, w := range windows {
				startsAt := fmt.Sprint(w["startsAt"])
				endsAt := fmt.Sprint(w["endsAt"])
				d1, h1 := dateAndHHMMFromISO(startsAt)
				d2, h2 := dateAndHHMMFromISO(endsAt)
				if d1 == date && d2 == date {
					possible = append(possible, fmt.Sprintf("%s-%s", h1, h2))
				}
				if d1 == date && d2 == date && h1 == fromTime && h2 == toTime {
					selected = w
					break
				}
			}
			if selected == nil {
				return fmt.Errorf(tr("Slot %s-%s not found for %s. Available: %s", "Nie znaleziono slotu %s-%s dla %s. Dostępne: %s"),
					fromTime, toTime, date, strings.Join(possible, ", "))
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
				return err
			}
			return printJSON(map[string]any{
				"reserved": true,
				"slot": map[string]any{
					"startsAt":       selected["startsAt"],
					"endsAt":         selected["endsAt"],
					"deliveryMethod": selected["deliveryMethod"],
					"warehouse":      selected["warehouse"],
				},
				"apiResponse": result,
			})
		},
	}
	c.Flags().StringVar(&date, "date", "", tr("Date YYYY-MM-DD", "Data YYYY-MM-DD"))
	c.Flags().StringVar(&fromTime, "from-time", "", tr("Start time HH:MM", "Godzina startu HH:MM"))
	c.Flags().StringVar(&toTime, "to-time", "", tr("End time HH:MM", "Godzina końca HH:MM"))
	c.Flags().StringVar(&shippingFile, "shipping-address-file", "", tr("Optional address JSON.", "Opcjonalny JSON z adresem."))
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("date")
	_ = c.MarkFlagRequired("from-time")
	_ = c.MarkFlagRequired("to-time")
	return c
}

func newReservationPlanCmd() *cobra.Command {
	var payloadFile, userID string
	c := &cobra.Command{
		Use:   "plan",
		Short: tr("Plan cart reservation from JSON payload.", "Zaplanuj rezerwację koszyka z payloadu JSON."),
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
			payload, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf(tr("Payload file must contain a JSON object.", "Plik payload musi zawierać obiekt JSON."))
			}
			path := fmt.Sprintf("/app/commerce/api/v2/users/%s/cart/reservation", uid)
			result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
				Data:       payload,
				DataFormat: httpclient.FormatJSON,
			})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&payloadFile, "payload-file", "", tr("JSON payload like /cart/reservation", "JSON jak w /cart/reservation"))
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("payload-file")
	return c
}

func newReservationStatusCmd() *cobra.Command {
	var userID string
	var pageIndex, pageSize int
	c := &cobra.Command{
		Use:   "status",
		Short: tr("User order/reservation status.", "Status zamówień/rezerwacji użytkownika."),
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
	c.Flags().IntVar(&pageSize, "page-size", 20, "")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newReservationCancelCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "cancel",
		Short: tr("Cancel active cart reservation.", "Anuluj aktywną rezerwację koszyka."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart/reservation", uid)
			result, err := httpclient.RequestJSON(s, "DELETE", path, httpclient.RequestOpts{})
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}
