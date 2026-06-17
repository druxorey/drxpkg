package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/tui"
)

func main() {
	if os.Getuid() == 0 {
		fmt.Println("drxpkg should not be run as root.")
		os.Exit(1)
	}

	showHelp := flag.Bool("h", false, "Show help information")
	flag.Parse()

	if *showHelp {
		fmt.Println("drxpkg - Simple Arch Linux / AUR package manager TUI and tracker")
		fmt.Println("Usage: drxpkg [options]")
		fmt.Println("Options:")
		fmt.Println("  -h       Show this help information")
		os.Exit(0)
	}

	conf, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	appUI, err := tui.New(conf)
	if err != nil {
		fmt.Printf("Initialization error: %v\n", err)
		os.Exit(1)
	}

	if err := appUI.Start(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}
