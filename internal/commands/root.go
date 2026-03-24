package commands

import "github.com/spf13/cobra"

// Execute runs the root command (for main).
func Execute() error {
	return NewRootCmd().Execute()
}

// NewRootCmd builds the full CLI tree (parity with frisco_cli.py argparse).
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "frisco",
		Short: "CLI do obsługi endpointów Frisco znalezionych w HAR (XHR).",
		Long:  "Narzędzie do sesji, importu HAR, wywołań XHR oraz operacji commerce API — port z frisco_cli.py.",
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newSessionCmd(),
		newHarCmd(),
		newXHRCmd(),
		newProductsCmd(),
		newCartCmd(),
		newReservationCmd(),
		newAccountCmd(),
		newOrdersCmd(),
		newAuthCmd(),
	)
	return root
}
