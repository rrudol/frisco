// Package commands implements the frisco CLI command tree.
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Execute runs the root command (for main).
func Execute() error {
	return NewRootCmd().Execute()
}

// NewRootCmd builds the full CLI command tree.
func NewRootCmd() *cobra.Command {
	format := outputFormat
	root := &cobra.Command{
		Use:   "frisco",
		Short: "CLI for Frisco.pl grocery delivery API.",
		Long:  "Session management, product search, cart, orders, reservations and account operations.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "table"
			}
			if format != "table" && format != "json" {
				return fmt.Errorf(
					"Unsupported --format: %s (use table or json)",
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
		&format,
		"format",
		"table",
		"Output format: table or json.",
	)

	root.AddCommand(
		newSessionCmd(),
		newProductsCmd(),
		newCartCmd(),
		newReservationCmd(),
		newAccountCmd(),
		newMCPCmd(),
	)
	return root
}
