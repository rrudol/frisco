package main

import (
	"fmt"
	"os"

	"github.com/rrudol/frisco/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "BŁĄD: %v\n", err)
		os.Exit(1)
	}
}
