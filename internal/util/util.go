// Package util provides general utility helper functions used across the application.
package util

import (
	"fmt"
)

// PrintSuccess prints a formatted success message with colored prefix.
func PrintSuccess(format string, a ...any) {
	if len(format) > 0 && format[0] == '\n' {
		fmt.Print("\n")
		format = format[1:]
	}
	fmt.Printf("\033[1;32m[SUCCESS]\033[0m " + format, a...)
}

// PrintError prints a formatted error message with colored prefix.
func PrintError(format string, a ...any) {
	if len(format) > 0 && format[0] == '\n' {
		fmt.Print("\n")
		format = format[1:]
	}
	fmt.Printf("\033[1;31m[ERROR]\033[0m " + format, a...)
}
