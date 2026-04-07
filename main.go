package main

import (
	"os"

	"github.com/Lalcs/a2ahoy/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
