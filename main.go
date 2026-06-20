package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/tui"
	"github.com/druxorey/drxpkg/internal/util"
)

func main() {
	if os.Getuid() == 0 {
		util.PrintError("drxpkg should not be run as root.\n")
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
		util.PrintError("Could not load config: %v\n", err)
		os.Exit(1)
	}

	appUI, err := tui.New(conf)
	if err != nil {
		util.PrintError("Could not initialize: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Print("\033[H\033[2J")
	}

	if err := appUI.Start(); err != nil {
		util.PrintError("Could not run the application: %v\n", err)
		os.Exit(1)
	}
}
