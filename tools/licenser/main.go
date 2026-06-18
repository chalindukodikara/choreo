// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Flag variables

var (
	flagCheckOnly = flag.Bool(
		"check-only",
		false,
		"Only verify headers (exit 1 if non-compliant)",
	)
	flagHolder = flag.String(
		"c",
		"",
		`Copyright holder, e.g. "The OpenChoreo Authors"`,
	)
)

// Constants
const licenseID = "Apache-2.0"

// Header detection / generation

func shortHeader(year, holder string) string {
	return fmt.Sprintf(
		"// Copyright %s %s\n// SPDX-License-Identifier: %s",
		year, holder, licenseID,
	)
}

// File helpers

func isGoFile(path string) bool { return filepath.Ext(path) == ".go" }

func isPythonFile(path string) bool { return filepath.Ext(path) == ".py" }

func isSupportedSourceFile(path string) bool { return isGoFile(path) || isPythonFile(path) }

func commentPrefix(path string) string {
	switch filepath.Ext(path) {
	case ".py":
		return "#"
	default:
		return "//"
	}
}

func headerForPath(path, header string) string {
	prefix := commentPrefix(path)
	if prefix == "//" {
		return header
	}
	return strings.ReplaceAll(header, "// ", prefix+" ")
}

func headerRegexes(path string) (*regexp.Regexp, *regexp.Regexp, *regexp.Regexp) {
	return headerRegexesForPrefix(commentPrefix(path))
}

func headerRegexesForPrefix(commentPrefix string) (*regexp.Regexp, *regexp.Regexp, *regexp.Regexp) {
	prefix := regexp.QuoteMeta(commentPrefix)
	return regexp.MustCompile(`^` + prefix + ` Copyright (\d{4}) (.+)$`),
		regexp.MustCompile(`^` + prefix + ` SPDX-License-Identifier: (Apache-2\.0)$`),
		regexp.MustCompile(`^` + prefix + ` SPDX-License-Identifier: .+$`)
}

func stripCommentPrefixes(path string) []string {
	prefix := commentPrefix(path)
	if prefix == "#" {
		return []string{"#", "//"}
	}
	return []string{"//", "#"}
}

func matchesCopyrightLine(path, line string) bool {
	for _, prefix := range stripCommentPrefixes(path) {
		reCopyrightForPrefix, _, _ := headerRegexesForPrefix(prefix)
		if reCopyrightForPrefix.MatchString(line) {
			return true
		}
	}
	return false
}

func matchesSPDXLine(path, line string) bool {
	for _, prefix := range stripCommentPrefixes(path) {
		_, _, reAnySPDXForPrefix := headerRegexesForPrefix(prefix)
		if reAnySPDXForPrefix.MatchString(line) {
			return true
		}
	}
	return false
}

func hasValidHeader(path, holder string) (bool, error) {
	return hasValidHeaderWithYear(path, holder, fmt.Sprint(time.Now().Year()), isNewGitFile(path))
}

func hasValidHeaderWithYear(path, holder, year string, requireYear bool) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	var lines []string
	for scan.Scan() {
		line := scan.Text()

		// Skip any leading blank lines
		if strings.TrimSpace(line) == "" && len(lines) == 0 {
			continue
		}
		lines = append(lines, line)

		// We need three lines: copyright, SPDX, blank
		if len(lines) == 3 {
			break
		}
	}

	// Must have exactly the three expected lines
	if len(lines) < 3 {
		return false, nil
	}

	reCopyrightForPath, reSPDXForPath, _ := headerRegexes(path)
	m1 := reCopyrightForPath.FindStringSubmatch(lines[0])
	m2 := reSPDXForPath.FindStringSubmatch(lines[1])
	blank := strings.TrimSpace(lines[2]) == ""

	if m1 == nil || m2 == nil || !blank {
		return false, nil
	}

	if requireYear && m1[1] != year {
		return false, nil
	}

	return m1[2] == holder && m2[1] == licenseID, nil
}

func isNewGitFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	rootCmd := exec.Command("git", "-C", filepath.Dir(absPath), "rev-parse", "--show-toplevel")
	rootOut, err := rootCmd.Output()
	if err != nil {
		return false
	}

	repoRoot := strings.TrimSpace(string(rootOut))
	repoPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return false
	}

	cmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain", "--untracked-files=all", "--", repoPath)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	status := strings.TrimSpace(string(out))
	return strings.HasPrefix(status, "A") || strings.HasPrefix(status, "??")
}

// stripExistingHeader removes an existing copyright/SPDX header from the
// beginning of src so that a correct header can be prepended without duplication.
func stripExistingHeader(src []byte) []byte {
	return stripExistingHeaderForPath("", src)
}

func stripExistingHeaderForPath(path string, src []byte) []byte {
	lines := strings.Split(string(src), "\n")

	// Skip leading blank lines
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}

	// Check for a copyright or SPDX line at the top
	hasCopyright := i < len(lines) && matchesCopyrightLine(path, lines[i])
	hasSPDXOnly := !hasCopyright && i < len(lines) && matchesSPDXLine(path, lines[i])

	if hasCopyright {
		// Check if followed by SPDX line (full header pair)
		if i+1 < len(lines) && matchesSPDXLine(path, lines[i+1]) {
			i += 2
		} else {
			// Copyright-only header (missing SPDX)
			i++
		}
	} else if hasSPDXOnly {
		// SPDX-only header (missing copyright)
		i++
	} else {
		return src
	}

	// Strip the blank line that follows the header
	if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return []byte(strings.Join(lines[i:], "\n"))
}

func prependHeader(path, header string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src = stripExistingHeaderForPath(path, src)
	return os.WriteFile(path, append([]byte(headerForPath(path, header)+"\n\n"), src...), 0o644)
}

// Core processing loop

func process(path, header, holder string, fix bool) (changed bool, err error) {
	ok, err := hasValidHeader(path, holder)
	if err != nil || ok {
		return false, err
	}
	if !fix {
		return true, nil // non-compliant
	}
	return true, prependHeader(path, header)
}

func walk(root, header, holder string, fix bool) ([]string, error) {
	var nonCompliant []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isSupportedSourceFile(p) {
			return err
		}
		changed, err := process(p, header, holder, fix)
		if err != nil {
			return err
		}
		if changed {
			nonCompliant = append(nonCompliant, p)
		}
		return nil
	})
	return nonCompliant, err
}

// CLI

const usageText = `
Licenser is a tool to enforce short SPDX license headers in source files.

OVERVIEW
  licenser verifies that each source file starts with a standard two-line header:
    // Copyright <YEAR> <HOLDER>
    // SPDX-License-Identifier: <LICENSE>

USAGE
  go run ./tools/licenser/main.go [flags] <directories or files>

FLAGS
  -check-only    Only report non-compliant files; do not modify them (default: false)
  -c <holder>    Copyright holder (e.g. "The OpenChoreo Authors")

EXAMPLES
  # Check license compliance in all Go files under the current directory
  go run ./tools/licenser/main.go -check-only -c "The OpenChoreo Authors" .

  # Add/fix license headers in place
  go run ./tools/licenser/main.go -c "The OpenChoreo Authors" .

LEARN MORE
  SPDX License Identifiers: https://spdx.org/licenses/

Note: Currently .go files and .py files are processed.
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usageText) }
	flag.Parse()

	if flag.NArg() == 0 || (*flagHolder == "" && !*flagCheckOnly) {
		flag.Usage()
		os.Exit(0)
	}

	header := shortHeader(fmt.Sprint(time.Now().Year()), *flagHolder)
	mode := "CHECK"
	if !*flagCheckOnly {
		mode = "FIX"
	}
	fmt.Printf("Running in %s mode (apache license)\n", mode)

	var offending []string
	for _, dir := range flag.Args() {
		files, err := walk(dir, header, *flagHolder, !*flagCheckOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error scanning %s: %v\n", dir, err)
			os.Exit(2)
		}
		offending = append(offending, files...)
	}

	if *flagCheckOnly {
		if len(offending) > 0 {
			fmt.Println("❌ Missing or invalid headers:")
			for _, f := range offending {
				fmt.Printf(" • %s\n", f)
			}
			os.Exit(1)
		}
		fmt.Println("✅ All files have valid headers.")
	} else {
		if len(offending) > 0 {
			fmt.Println("🛠 Added headers to:")
			for _, f := range offending {
				fmt.Printf(" • %s\n", f)
			}
		} else {
			fmt.Println("✅ No changes needed – all headers already valid.")
		}
	}
}
