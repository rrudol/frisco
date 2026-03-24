package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/i18n"
)

// Execute runs the root command (for main).
func Execute() error {
	env := map[string]string{
		"FRISCO_LANG": os.Getenv("FRISCO_LANG"),
		"LC_ALL":      os.Getenv("LC_ALL"),
		"LC_MESSAGES": os.Getenv("LC_MESSAGES"),
		"LANG":        os.Getenv("LANG"),
	}
	i18n.Set(i18n.DetectFromArgsEnv(os.Args[1:], env))
	return NewRootCmd().Execute()
}

// NewRootCmd builds the full CLI command tree.
func NewRootCmd() *cobra.Command {
	lang := string(i18n.Current())
	format := outputFormat
	root := &cobra.Command{
		Use: "frisco",
		Short: tr(
			"CLI for Frisco.pl grocery delivery API.",
			"CLI do obsługi API dostawy Frisco.pl.",
		),
		Long: tr(
			"Session management, product search, cart, orders, reservations and account operations.",
			"Zarządzanie sesją, wyszukiwanie produktów, koszyk, zamówienia, rezerwacje i operacje konta.",
		),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			parsed, ok := i18n.Parse(lang)
			if !ok {
				return fmt.Errorf(
					tr("Unsupported --lang: %s (use en or pl)", "Nieobsługiwany --lang: %s (użyj en albo pl)"),
					lang,
				)
			}
			i18n.Set(parsed)
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "table"
			}
			if format != "table" && format != "json" {
				return fmt.Errorf(
					tr("Unsupported --format: %s (use table or json)", "Nieobsługiwany --format: %s (użyj table albo json)"),
					format,
				)
			}
			outputFormat = format
			return nil
		},
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(
		&lang,
		"lang",
		string(i18n.Current()),
		tr("Output language: en or pl.", "Język komunikatów: en albo pl."),
	)
	root.PersistentFlags().StringVar(
		&format,
		"format",
		"table",
		tr("Output format: table or json.", "Format wyjścia: table albo json."),
	)

	root.AddCommand(
		newSessionCmd(),
		newProductsCmd(),
		newCartCmd(),
		newReservationCmd(),
		newAccountCmd(),
		newOrdersCmd(),
		newMCPCmd(),
	)
	return root
}
