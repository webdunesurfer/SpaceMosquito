package main

import (
	"os"

	"github.com/vkh/spacemosquito/internal/cliapp"
)

func main() {
	args := append([]string{os.Args[0], "serve"}, os.Args[1:]...)
	os.Exit(cliapp.Run(args))
}
