package main

import (
	"fmt"
	"os"

	"github.com/rrudol/frisco/internal/commands"
	"github.com/rrudol/frisco/internal/i18n"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", i18n.T("ERROR", "BŁĄD"), err)
		os.Exit(1)
	}
}
