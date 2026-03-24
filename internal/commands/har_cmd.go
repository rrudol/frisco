package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rrudol/frisco/internal/session"
)

func newHarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "har",
		Short: tr("HAR import and management.", "Import/obsługa HAR."),
	}
	cmd.AddCommand(newHarImportCmd())
	return cmd
}

func newHarImportCmd() *cobra.Command {
	var path string
	c := &cobra.Command{
		Use:   "import",
		Short: tr("Import XHR endpoints from HAR.", "Importuj endpointy XHR z HAR."),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoints, err := session.ParseHarXHR(path)
			if err != nil {
				return err
			}
			s, err := session.Load()
			if err != nil {
				return err
			}
			s.Endpoints = endpoints
			s.HarPath = path
			for _, ep := range endpoints {
				if uid := session.ExtractUserID(ep.URL); uid != "" {
					s.UserID = uid
					break
				}
			}
			if err := session.Save(s); err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				tr("Imported XHR: %d unique endpoints.\n", "Zaimportowano XHR: %d unikalnych endpointów.\n"),
				len(endpoints),
			)
			return err
		},
	}
	c.Flags().StringVar(&path, "path", session.DefaultHARPath(), tr("Path to HAR file.", "Ścieżka do pliku HAR."))
	return c
}
