package main

import (
	"os"

	"github.com/rxtech-lab/rvmm/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
