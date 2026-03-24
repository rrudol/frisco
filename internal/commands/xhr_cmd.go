package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newXHRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xhr",
		Short: tr("Low-level access to XHR endpoints.", "Niskopoziomowy dostęp do endpointów XHR."),
	}
	cmd.AddCommand(newXHRListCmd(), newXHRCallCmd())
	return cmd
}

func newXHRListCmd() *cobra.Command {
	var contains string
	c := &cobra.Command{
		Use:   "list",
		Short: tr("List imported XHR endpoints.", "Wylistuj zaimportowane endpointy XHR."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			endpoints := s.Endpoints
			if len(endpoints) == 0 {
				_, _ = fmt.Fprintln(
					cmd.OutOrStdout(),
					tr("No endpoints in session. Run: har import", "Brak endpointów w sesji. Uruchom: har import"),
				)
				return nil
			}
			filtered := endpoints
			if contains != "" {
				needle := strings.ToLower(contains)
				var next []session.Endpoint
				for _, ep := range endpoints {
					if strings.Contains(strings.ToLower(ep.PathTemplate), needle) ||
						strings.Contains(strings.ToLower(ep.Method), needle) {
						next = append(next, ep)
					}
				}
				filtered = next
			}
			out := cmd.OutOrStdout()
			for _, ep := range filtered {
				q := ""
				if ep.HasQuery {
					q = "?"
				}
				_, _ = fmt.Fprintf(out, "%-6s %s%s\n", ep.Method, ep.PathTemplate, q)
			}
			_, _ = fmt.Fprintf(out, tr("\nTotal: %d\n", "\nRazem: %d\n"), len(filtered))
			return nil
		},
	}
	c.Flags().StringVar(&contains, "contains", "", tr("Filter by path/method fragment.", "Filtr po fragmencie ścieżki/metody."))
	return c
}

func newXHRCallCmd() *cobra.Command {
	var (
		method, pathOrURL, dataStr, dataFormat string
		query                                  []string
		headers                                []string
	)
	c := &cobra.Command{
		Use:   "call",
		Short: tr("Call any endpoint.", "Wywołaj dowolny endpoint."),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			payload, err := parseJSONOrKV(dataStr)
			if err != nil {
				return err
			}
			extra := map[string]string{}
			for _, h := range headers {
				idx := strings.IndexByte(h, ':')
				if idx < 0 {
					return fmt.Errorf(
						tr("Bad header: %s. Expected Key: Value", "Zły nagłówek: %s. Oczekiwane Key: Value"),
						h,
					)
				}
				extra[strings.TrimSpace(h[:idx])] = strings.TrimSpace(h[idx+1:])
			}
			opts := httpclient.RequestOpts{
				Query:        query,
				ExtraHeaders: extra,
			}
			if payload != nil {
				format := dataFormat
				if format == "auto" {
					trim := strings.TrimSpace(dataStr)
					switch {
					case strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "["):
						format = "json"
					case strings.Contains(trim, "="):
						format = "form"
					default:
						format = "raw"
					}
				}
				switch format {
				case "json":
					opts.Data = payload
					opts.DataFormat = httpclient.FormatJSON
				case "form":
					opts.Data = payload
					opts.DataFormat = httpclient.FormatForm
				case "raw":
					strPayload, ok := payload.(string)
					if !ok {
						return errors.New(tr("For data_format=raw provide string.", "Dla data_format=raw podaj string."))
					}
					opts.Data = strPayload
					opts.DataFormat = httpclient.FormatRaw
				default:
					return fmt.Errorf(tr("unsupported --data-format: %s", "nieobsługiwany --data-format: %s"), format)
				}
			}
			result, err := httpclient.RequestJSON(s, method, pathOrURL, opts)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&method, "method", "", tr("HTTP method, e.g. GET/POST/PUT/DELETE", "HTTP method, np. GET/POST/PUT/DELETE"))
	c.Flags().StringVar(&pathOrURL, "path-or-url", "", tr("Relative path or full URL.", "Ścieżka względna lub pełny URL."))
	c.Flags().StringArrayVar(&query, "query", nil, tr("Query parameter key=value (repeatable).", "Parametr query key=value (powtarzalny)."))
	c.Flags().StringArrayVar(&headers, "header", nil, tr("Header Key: Value (repeatable).", "Nagłówek Key: Value (powtarzalny)."))
	c.Flags().StringVar(&dataStr, "data", "", tr("JSON body or key=value&k2=v2", "JSON body albo key=value&k2=v2"))
	c.Flags().StringVar(&dataFormat, "data-format", "auto", tr("Body format: auto/json/form/raw", "Format body: auto/json/form/raw"))
	_ = c.MarkFlagRequired("method")
	_ = c.MarkFlagRequired("path-or-url")
	return c
}
