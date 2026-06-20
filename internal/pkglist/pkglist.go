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

const PackagesFileName = "drxboot.packages"

var Categories = []string{"server", "minimal", "desktop", "new"}

type PackageMap map[string][]string

func NewPackageMap() PackageMap {
	pm := make(PackageMap)
	for _, category := range Categories {
		pm[category] = []string{}
	}
	return pm
}

func GetFilePath(customPath string) (string, error) {
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

	return filepath.Join(customPath, PackagesFileName), nil
}

func Load(customPath string) (PackageMap, error) {
	packages := NewPackageMap()
	filePath, err := GetFilePath(customPath)
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
		matched := false

		for _, cat := range Categories {
			if strings.HasPrefix(line, fmt.Sprintf("%s_packages=(", cat)) {
				currentCat = cat
				matched = true
				break
			}
		}

		if matched {
			continue
		} else if line == ")" {
			currentCat = ""
		} else if currentCat != "" && line != "" && !strings.HasPrefix(line, "#") {
			packages[currentCat] = append(packages[currentCat], line)
		}
	}
	return packages, scanner.Err()
}

func Save(customPath string, packages PackageMap) error {
	filePath, err := GetFilePath(customPath)
	if err != nil {
		return err
	}
	dirPath := filepath.Dir(filePath)

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

	for _, cat := range Categories {
		_, _ = fmt.Fprintf(writer, "%s_packages=(\n", cat)

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
	for _, cat := range Categories {
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

func AddPackage(customPath string, pkg string) error {
	allPackages, err := Load(customPath)
	if err != nil {
		return err
	}

	_, _, found := FindPackageLocation(pkg, allPackages)
	if found {
		return nil
	}

	allPackages["new"] = append(allPackages["new"], pkg)
	return Save(customPath, allPackages)
}

func RemovePackage(customPath string, pkg string) error {
	allPackages, err := Load(customPath)
	if err != nil {
		return err
	}

	cat, fullPkgName, found := FindPackageLocation(pkg, allPackages)
	if !found {
		return nil
	}

	allPackages[cat] = removeValue(allPackages[cat], fullPkgName)
	return Save(customPath, allPackages)
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
