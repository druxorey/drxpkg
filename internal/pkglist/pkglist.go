// Package pkglist provides functionality to read, write, and manipulate the tracked packages list file.
package pkglist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"github.com/druxorey/drxpkg/internal/util"
)

const DefaultPackagesFileName = "packages.list"

var Categories = []string{"server", "minimal", "desktop", "new"}

type PackageMap map[string][]string

func NewPackageMap() PackageMap {
	pm := make(PackageMap)
	pm["new_packages"] = []string{}
	return pm
}

func parseBashArrayName(line string) (string, bool) {
	parts := strings.SplitN(line, "=(", 2)
	if len(parts) != 2 {
		return "", false
	}
	name := strings.TrimSpace(parts[0])
	if len(name) == 0 {
		return "", false
	}
	for i, r := range name {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return "", false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return "", false
			}
		}
	}
	return name, true
}

func GetFilePath(customPath string, fileName string) (string, error) {
	if fileName == "" {
		fileName = DefaultPackagesFileName
	}
	if customPath == "" {
		customPath = os.Getenv("PACKAGES_PATH")
	}
	if customPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		customPath = homeDir
	}

	if strings.HasPrefix(customPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			customPath = filepath.Join(homeDir, customPath[2:])
		}
	} else if customPath == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			customPath = homeDir
		}
	}
	customPath = os.ExpandEnv(customPath)

	return filepath.Join(customPath, fileName), nil
}

func Load(customPath string, fileName string) (PackageMap, error) {
	packages := NewPackageMap()
	filePath, err := GetFilePath(customPath, fileName)
	if err != nil {
		return packages, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return packages, nil
		}
		return packages, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			util.PrintError("Could not close file: %v", err)
		}
	}()

	scanner := bufio.NewScanner(file)
	var currentCat string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == ")" {
			currentCat = ""
			continue
		}

		if cat, ok := parseBashArrayName(line); ok {
			currentCat = cat
			if _, exists := packages[currentCat]; !exists {
				packages[currentCat] = []string{}
			}
			continue
		}

		if currentCat != "" {
			packages[currentCat] = append(packages[currentCat], line)
		}
	}
	return packages, scanner.Err()
}

func getOriginalArrayOrder(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	var order []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if name, ok := parseBashArrayName(line); ok {
			if !seen[name] {
				seen[name] = true
				order = append(order, name)
			}
		}
	}
	return order, scanner.Err()
}

func Save(customPath string, fileName string, packages PackageMap) error {
	filePath, err := GetFilePath(customPath, fileName)
	if err != nil {
		return err
	}
	dirPath := filepath.Dir(filePath)

	// Get original order of arrays before overwriting the file.
	originalOrder, _ := getOriginalArrayOrder(filePath)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			util.PrintError("Could not save file: %v", err)
		}
	}()

	writer := bufio.NewWriter(file)
	_, _ = writer.WriteString("#!/bin/bash\n\n")

	var finalKeys []string
	seenKeys := make(map[string]bool)

	// Keep the original order of keys, excluding "new_packages"
	for _, key := range originalOrder {
		if key != "new_packages" {
			if _, exists := packages[key]; exists {
				finalKeys = append(finalKeys, key)
				seenKeys[key] = true
			}
		}
	}

	// Add any new keys alphabetically (excluding "new_packages")
	var newKeys []string
	for k := range packages {
		if k != "new_packages" && !seenKeys[k] {
			newKeys = append(newKeys, k)
		}
	}
	sort.Strings(newKeys)
	finalKeys = append(finalKeys, newKeys...)

	// Always append "new_packages" at the end
	if _, exists := packages["new_packages"]; exists {
		finalKeys = append(finalKeys, "new_packages")
	}

	for _, cat := range finalKeys {
		_, _ = fmt.Fprintf(writer, "%s=(\n", cat)

		pkgMap := make(map[string]bool)
		for _, p := range packages[cat] {
			if strings.TrimSpace(p) != "" {
				pkgMap[p] = true
			}
		}

		var sortedPackages []string
		for p := range pkgMap {
			sortedPackages = append(sortedPackages, p)
		}
		sort.Strings(sortedPackages)

		for _, pkg := range sortedPackages {
			_, _ = fmt.Fprintf(writer, "\t%s\n", pkg)
		}
		_, _ = writer.WriteString(")\n\n")
	}
	return writer.Flush()
}

func FindPackageLocation(pkgName string, allPackages PackageMap) (string, string, bool) {
	suffixes := []string{"", "-git", "-bin"}
	
	var keys []string
	for k := range allPackages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, cat := range keys {
		pkgList := allPackages[cat]
		for _, sfx := range suffixes {
			fullPkgName := pkgName + sfx
			if slices.Contains(pkgList, fullPkgName) {
				return cat, fullPkgName, true
			}
		}
		for _, sfx := range suffixes {
			if sfx != "" && strings.HasSuffix(pkgName, sfx) {
				baseName := strings.TrimSuffix(pkgName, sfx)
				if slices.Contains(pkgList, baseName) {
					return cat, baseName, true
				}
			}
		}
	}
	return "", "", false
}

func AddPackage(customPath string, fileName string, pkg string) error {
	allPackages, err := Load(customPath, fileName)
	if err != nil {
		return err
	}

	_, _, found := FindPackageLocation(pkg, allPackages)
	if found {
		return nil
	}

	allPackages["new_packages"] = append(allPackages["new_packages"], pkg)
	return Save(customPath, fileName, allPackages)
}

func RemovePackage(customPath string, fileName string, pkg string) error {
	allPackages, err := Load(customPath, fileName)
	if err != nil {
		return err
	}

	cat, fullPkgName, found := FindPackageLocation(pkg, allPackages)
	if !found {
		return nil
	}

	allPackages[cat] = removeValue(allPackages[cat], fullPkgName)
	return Save(customPath, fileName, allPackages)
}

func removeValue(slice []string, value string) []string {
	var result []string
	for _, v := range slice {
		if v != value {
			result = append(result, v)
		}
	}
	return result
}
