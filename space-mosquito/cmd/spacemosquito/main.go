package main

import (
	"os"

	"github.com/vkh/spacemosquito/internal/cliapp"
)

func main() {
	os.Exit(cliapp.Run(os.Args))
}
