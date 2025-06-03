// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAddLicense(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AddLicense Suite")
}

var _ = Describe("AddLicense tool", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "addlicense-test-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	writeFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		Expect(err).ToNot(HaveOccurred())
		return path
	}

	Context("hasValidShortHeader", func() {
		It("detects a valid short license header", func() {
			content := `// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package main
`
			f := writeFile("valid.go", content)
			ok, err := hasValidShortHeader(f)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("detects missing or invalid header", func() {
			content := `// Some other header
package main
`
			f := writeFile("invalid.go", content)
			ok, err := hasValidShortHeader(f)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("returns false for empty files", func() {
			f := writeFile("empty.go", "")
			ok, err := hasValidShortHeader(f)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	Context("processFile", func() {
		var header string

		BeforeEach(func() {
			header = getShortHeader("2025", "The OpenChoreo Authors", "apache")
		})

		It("does not update compliant file", func() {
			content := header + "\n\npackage main\n"
			f := writeFile("already.go", content)
			updated, err := processFile(f, header)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeFalse())

			data, err := os.ReadFile(f)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(data)).To(Equal(content))
		})

		It("in check-only mode returns true for non-compliant file", func() {
			content := `package main
`
			f := writeFile("noheader.go", content)

			*checkOnly = true
			updated, err := processFile(f, header)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeTrue())
			*checkOnly = false
		})

		It("prepends header in update mode for non-compliant file", func() {
			content := `package main
`
			f := writeFile("noheader.go", content)

			updated, err := processFile(f, header)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeTrue())

			data, err := os.ReadFile(f)
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(string(data), header)).To(BeTrue())
		})
	})

	Context("walkDir", func() {
		var header string

		BeforeEach(func() {
			header = getShortHeader("2025", "The OpenChoreo Authors", "apache")
		})

		It("walks directory and returns non-compliant go files", func() {
			// Compliant file
			writeFile("valid.go", header+"\n\npackage main\n")

			// Non-compliant file
			writeFile("invalid.go", "package main\n")

			// Non-Go file
			writeFile("notgo.txt", "random text")

			nonCompliant, err := walkDir(tmpDir, header)
			Expect(err).ToNot(HaveOccurred())
			Expect(nonCompliant).To(ContainElement(filepath.Join(tmpDir, "invalid.go")))
			Expect(nonCompliant).ToNot(ContainElement(filepath.Join(tmpDir, "valid.go")))
			Expect(nonCompliant).ToNot(ContainElement(filepath.Join(tmpDir, "notgo.txt")))
		})
	})
})
