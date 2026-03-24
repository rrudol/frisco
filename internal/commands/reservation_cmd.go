package commands

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newReservationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservation",
		Short: "Reservation planning and status.",
	}
	cmd.AddCommand(
		newReservationDeliveryOptionsCmd(),
		newReservationCalendarCmd(),
		newReservationSlotsCmd(),
		newReservationReserveCmd(),
		newReservationPlanCmd(),
		newReservationCancelCmd(),
	)
	return cmd
}

func newReservationDeliveryOptionsCmd() *cobra.Command {
	var postcode string
	c := &cobra.Command{
		Use:   "delivery-options",
		Short: "Delivery/payment options by postcode.",
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
		Short: "Available delivery windows for address.",
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
				return errors.New("address file must contain a JSON object")
			}
			body := map[string]any{"shippingAddress": shippingAddress}
			var path string
			if date != "" {
				parts := strings.Split(date, "-")
				if len(parts) != 3 {
					return errors.New("date must be in format YYYY-M-D or YYYY-MM-DD")
				}
				y, err1 := strconv.Atoi(parts[0])
				m, err2 := strconv.Atoi(parts[1])
				d, err3 := strconv.Atoi(parts[2])
				if err1 != nil || err2 != nil || err3 != nil {
					return errors.New("date must be in format YYYY-M-D or YYYY-MM-DD")
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
	c.Flags().StringVar(&shippingFile, "shipping-address-file", "", "JSON with shippingAddress.")
	c.Flags().StringVar(&date, "date", "", "Optional date YYYY-M-D")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("shipping-address-file")
	return c
}

// getShippingAddressFromAccount fetches the user's saved shipping addresses and
// returns the preferred (default/current) one, falling back to the first entry.
func getShippingAddressFromAccount(s *session.Session, userID string) (map[string]any, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", userID)
	data, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	list, ok := data.([]any)
	if !ok || len(list) == 0 {
		return nil, errors.New("no saved user addresses")
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
		return nil, errors.New("no saved user addresses")
	}
	if sa, ok := chosen["shippingAddress"].(map[string]any); ok {
		return sa, nil
	}
	return chosen, nil
}

// truthy returns true when v is a bool with value true.
func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

// nonEmptyStr converts v to a trimmed string and reports whether it is non-empty.
func nonEmptyStr(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s, s != ""
}

// extractDeliveryWindows recursively walks a calendar API response and collects
// unique delivery window objects (those with startsAt and endsAt), sorted by start time.
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

// extractReservableWindows is like extractDeliveryWindows but only returns windows
// that also have a deliveryMethod and warehouse set (required for the reserve API call).
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

// dateAndHHMMFromISO splits an ISO 8601 timestamp into a date part (YYYY-MM-DD)
// and an HH:MM time part.
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
		Short: "Get available delivery slots for upcoming days (including today).",
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
					return errors.New("address file must contain a JSON object")
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
			if !strings.EqualFold(outputFormat, "json") {
				return printSlotsTable(pretty)
			}
			return printJSON(map[string]any{"days": pretty})
		},
	}
	c.Flags().IntVar(&days, "days", 3, "How many upcoming days to check.")
	c.Flags().StringVar(&startDate, "start-date", "", "Start date YYYY-MM-DD (default: today).")
	c.Flags().StringVar(&shippingFile, "shipping-address-file", "", "Optional address JSON.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().BoolVar(&rawOut, "raw", false, "Return raw API response.")
	return c
}

func newReservationReserveCmd() *cobra.Command {
	var (
		date, fromTime, toTime, shippingFile, userID string
	)
	c := &cobra.Command{
		Use:   "reserve",
		Short: "Reserve slot by date and times (e.g. 06:00-07:00).",
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
					return errors.New("address file must contain a JSON object")
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
				return errors.New("no reservable slots for given date")
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
				return fmt.Errorf("slot %s-%s not found for %s. Available: %s",
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
			if strings.EqualFold(outputFormat, "json") {
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
			}
			_, startHH := dateAndHHMMFromISO(fmt.Sprint(selected["startsAt"]))
			_, endHH := dateAndHHMMFromISO(fmt.Sprint(selected["endsAt"]))
			fmt.Println("Reservation confirmed.")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "%s\t%s\n", "Date", date)
			fmt.Fprintf(w, "%s\t%s – %s\n", "Time", startHH, endHH)
			fmt.Fprintf(w, "%s\t%s\n", "Method", cellValue(selected["deliveryMethod"]))
			fmt.Fprintf(w, "%s\t%s\n", "Warehouse", cellValue(selected["warehouse"]))
			w.Flush()
			return nil
		},
	}
	c.Flags().StringVar(&date, "date", "", "Date YYYY-MM-DD")
	c.Flags().StringVar(&fromTime, "from-time", "", "Start time HH:MM")
	c.Flags().StringVar(&toTime, "to-time", "", "End time HH:MM")
	c.Flags().StringVar(&shippingFile, "shipping-address-file", "", "Optional address JSON.")
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
		Short: "Plan cart reservation from JSON payload.",
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
				return errors.New("payload file must contain a JSON object")
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
	c.Flags().StringVar(&payloadFile, "payload-file", "", "JSON payload like /cart/reservation")
	c.Flags().StringVar(&userID, "user-id", "", "")
	_ = c.MarkFlagRequired("payload-file")
	return c
}

// printSlotsTable renders a slice of day/slots maps as a per-day tabwriter table.
func printSlotsTable(days []map[string]any) error {
	for _, day := range days {
		date := cellValue(day["date"])
		fmt.Println(date)
		slots, _ := day["slots"].([]map[string]any)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "from\tto\tmethod\twarehouse")
		for _, slot := range slots {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				hhmm(slot["startsAt"]),
				hhmm(slot["endsAt"]),
				cellValue(slot["deliveryMethod"]),
				cellValue(slot["warehouse"]),
			)
		}
		w.Flush()
		fmt.Println()
	}
	return nil
}

func newReservationCancelCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel active cart reservation.",
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
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			fmt.Println("Reservation cancelled.")
			return nil
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}
