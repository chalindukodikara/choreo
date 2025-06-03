// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	checkOnly       = flag.Bool("check-only", false, "Only check for license headers, do not modify files")
	copyrightHolder = flag.String("c", "", "Copyright holder (e.g., 'The OpenChoreo Authors')")
	licenseType     = flag.String("l", "apache", "License type: apache or mit")
)

var (
	headerPattern = regexp.MustCompile(`^// Copyright (\d{4}) (.+)\n// SPDX-License-Identifier: (Apache-2.0|MIT)$`)
)

func getShortHeader(year, holder, license string) string {
	return fmt.Sprintf("// Copyright %s %s\n// SPDX-License-Identifier: %s", year, holder, licenseIdentifier(license))
}

func licenseIdentifier(license string) string {
	switch strings.ToLower(license) {
	case "mit":
		return "MIT"
	default:
		return "Apache-2.0"
	}
}

func isGoFile(path string) bool {
	return filepath.Ext(path) == ".go"
}

func hasValidShortHeader(file string) (bool, error) {
	f, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" && len(lines) == 0 {
			continue // skip leading blank lines
		}
		lines = append(lines, line)
		if len(lines) == 2 {
			break
		}
	}

	if len(lines) < 2 {
		return false, nil
	}

	matched := headerPattern.MatchString(strings.Join(lines, "\n"))
	return matched, nil
}

func prependHeader(filePath, header string) error {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := []byte(header + "\n\n" + string(input))
	return os.WriteFile(filePath, content, 0644)
}

func processFile(path string, header string) (updated bool, err error) {
	ok, err := hasValidShortHeader(path)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}

	if *checkOnly {
		return true, nil // mark as non-compliant
	}

	err = prependHeader(path, header)
	if err != nil {
		return false, err
	}
	return true, nil
}

func walkDir(dir string, header string) ([]string, error) {
	var nonCompliant []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isGoFile(path) {
			return nil
		}
		updated, err := processFile(path, header)
		if err != nil {
			return err
		}
		if updated {
			nonCompliant = append(nonCompliant, path)
		}
		return nil
	})
	return nonCompliant, err
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: addlicense [OPTIONS] <directories>

Options:
  -c string         Copyright holder (required in update mode)
  -l string         License type: apache or mit (default "apache")
  -check-only       If true, only checks for license headers, does not modify files

Examples:
  Check compliance:
    go run main.go -check-only .

  Add headers:
    go run main.go -c="The OpenChoreo Authors" .`)
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	if flag.NArg() == 0 || (*copyrightHolder == "" && !*checkOnly) {
		printUsage()
		os.Exit(1)
	}

	year := fmt.Sprintf("%d", getCurrentYear())
	header := getShortHeader(year, *copyrightHolder, *licenseType)

	var allNonCompliant []string
	for _, dir := range flag.Args() {
		files, err := walkDir(dir, header)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error walking directory %s: %v\n", dir, err)
			os.Exit(1)
		}
		allNonCompliant = append(allNonCompliant, files...)
	}

	if *checkOnly {
		if len(allNonCompliant) > 0 {
			fmt.Println("❌ Non-compliant files:")
			for _, f := range allNonCompliant {
				fmt.Println(" -", f)
			}
			fmt.Println("❌ Some files are missing valid short license headers.")
			os.Exit(1)
		}
		fmt.Println("✅ All files have valid short license headers.")
	} else {
		if len(allNonCompliant) > 0 {
			fmt.Println("✅ Added license headers to:")
			for _, f := range allNonCompliant {
				fmt.Println(" -", f)
			}
		} else {
			fmt.Println("✅ All files already have valid short license headers.")
		}
	}
}

func getCurrentYear() int {
	return time.Now().Year() // or use time.Now().Year() if you want dynamic year
}
