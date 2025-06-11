// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLicenseCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "License Check Suite")
}

var _ = Describe("License Header Checker", func() {
	var tmpDir string
	var header string
	holder := "The OpenChoreo Authors"
	license := "apache"

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "license-check-test")
		Expect(err).NotTo(HaveOccurred())

		header = getShortHeader(
			func() string {
				return time.Now().Format("2006")
			}(),
			holder,
			license,
		)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	writeFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		Expect(err).NotTo(HaveOccurred())
		return path
	}

	It("detects valid header", func() {
		content := header + `

package main

func main() {}
`
		path := writeFile("valid.go", content)

		valid, err := hasValidShortHeader(path, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(valid).To(BeTrue())
	})

	It("detects missing header", func() {
		content := `
package main

func main() {}
`
		path := writeFile("missing.go", content)

		valid, err := hasValidShortHeader(path, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(valid).To(BeFalse())
	})

	It("detects incorrect holder", func() {
		content := `// Copyright 2025 Someone Else
// SPDX-License-Identifier: Apache-2.0

package main

func main() {}
`
		path := writeFile("badholder.go", content)

		valid, err := hasValidShortHeader(path, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(valid).To(BeFalse())
	})

	It("adds header when missing", func() {
		content := `
package main

func main() {}
`
		path := writeFile("add.go", content)

		*checkOnly = false
		updated, err := processFile(path, header, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(updated).To(BeTrue())

		// Re-check
		valid, err := hasValidShortHeader(path, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(valid).To(BeTrue())
	})

	It("does not update file if checkOnly is true", func() {
		content := `
package main

func main() {}
`
		path := writeFile("checkonly.go", content)

		*checkOnly = true
		updated, err := processFile(path, header, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(updated).To(BeTrue()) // It's non-compliant

		// File should still be missing header
		valid, err := hasValidShortHeader(path, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(valid).To(BeFalse())
	})

	It("walks directory and finds non-compliant file", func() {
		content := `
package main

func main() {}
`
		writeFile("walk1.go", content)

		*checkOnly = true
		files, err := walkDir(tmpDir, header, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(HaveLen(1))
		Expect(strings.HasSuffix(files[0], "walk1.go")).To(BeTrue())
	})

	It("walks directory and updates file in non-check mode", func() {
		content := `
package main

func main() {}
`
		writeFile("walk2.go", content)

		*checkOnly = false
		files, err := walkDir(tmpDir, header, holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(HaveLen(1))
		Expect(strings.HasSuffix(files[0], "walk2.go")).To(BeTrue())

		valid, err := hasValidShortHeader(files[0], holder, license)
		Expect(err).NotTo(HaveOccurred())
		Expect(valid).To(BeTrue())
	})
})
