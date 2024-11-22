package main

import (
	"os"

	"github.com/beamlit/toolkit/cli"
)

func main() {
	err := cli.Execute()
	if err != nil {
		os.Exit(1)
	}
}
