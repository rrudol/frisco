package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/httpclient"
	"github.com/rrudol/frisco/internal/session"
)

func newXHRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xhr",
		Short: "Niskopoziomowy dostęp do endpointów XHR.",
	}
	cmd.AddCommand(newXHRListCmd(), newXHRCallCmd())
	return cmd
}

func newXHRListCmd() *cobra.Command {
	var contains string
	c := &cobra.Command{
		Use:   "list",
		Short: "Wylistuj zaimportowane endpointy XHR.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := session.Load()
			if err != nil {
				return err
			}
			endpoints := s.Endpoints
			if len(endpoints) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Brak endpointów w sesji. Uruchom: har import")
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
			_, _ = fmt.Fprintf(out, "\nRazem: %d\n", len(filtered))
			return nil
		},
	}
	c.Flags().StringVar(&contains, "contains", "", "Filtr po fragmencie ścieżki/metody.")
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
		Short: "Wywołaj dowolny endpoint.",
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
					return fmt.Errorf("Zły nagłówek: %s. Oczekiwane Key: Value", h)
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
						return fmt.Errorf("Dla data_format=raw podaj string.")
					}
					opts.Data = strPayload
					opts.DataFormat = httpclient.FormatRaw
				default:
					return fmt.Errorf("nieobsługiwany --data-format: %s", format)
				}
			}
			result, err := httpclient.RequestJSON(s, method, pathOrURL, opts)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&method, "method", "", "HTTP method, np. GET/POST/PUT/DELETE")
	c.Flags().StringVar(&pathOrURL, "path-or-url", "", "Ścieżka względna lub pełny URL.")
	c.Flags().StringArrayVar(&query, "query", nil, "Parametr query key=value (powtarzalny).")
	c.Flags().StringArrayVar(&headers, "header", nil, "Nagłówek Key: Value (powtarzalny).")
	c.Flags().StringVar(&dataStr, "data", "", "JSON body albo key=value&k2=v2")
	c.Flags().StringVar(&dataFormat, "data-format", "auto", "Format body: auto/json/form/raw")
	_ = c.MarkFlagRequired("method")
	_ = c.MarkFlagRequired("path-or-url")
	return c
}
