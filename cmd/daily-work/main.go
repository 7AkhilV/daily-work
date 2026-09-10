package main

import (
	"fmt"
	"os"

	"github.com/7AkhilV/daily-work/internal/cmd"
)

// Set at release build time: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	root := cmd.NewRoot()
	root.Version = version
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
