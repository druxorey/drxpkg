// Package util provides general utility helper functions used across the application.
package util

import (
	"fmt"
	"os"
)

// IndexOf returns an elements position in a slice
func IndexOf[T comparable](values []T, value T) int {
	for i, v := range values {
		if v == value {
			return i
		}
	}
	return -1
}

// Shell returns the users default shell
func Shell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh" // fallback
	}
	return shell
}

// MaxLenMapKey returns the length of the longest key string in a map of strings
func MaxLenMapKey(strMap map[string]string) int {
	maxLen := 0
	for k := range strMap {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	return maxLen
}

// UniqueStrings merges string slices
func UniqueStrings(strSlices ...[]string) []string {
	uniqueMap := map[string]bool{}

	for _, strSlice := range strSlices {
		for _, str := range strSlice {
			uniqueMap[str] = true
		}
	}

	result := make([]string, 0, len(uniqueMap))

	for key := range uniqueMap {
		result = append(result, key)
	}

	return result
}

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
