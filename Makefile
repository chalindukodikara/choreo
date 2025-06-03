# All the make targets are implemented in the make/*.mk files.
# To see all the available targets, run `make help`.

PROJECT_DIR := $(realpath $(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

#-----------------------------------------------------------------------------
# Makefile includes
#-----------------------------------------------------------------------------
include make/common.mk
include make/tools.mk
include make/golang.mk
include make/lint.mk
include make/docker.mk
include make/kube.mk
include make/helm.mk

.PHONY: license-check license-fix install-license-eye update-license-year

install-license-eye:
	@echo "Installing license-eye..."
	@go install github.com/apache/skywalking-eyes/cmd/license-eye@v0.7.0
	@echo "✅ license-eye installed"

update-license-year:
	@echo "Replacing '{{YEAR}}' with $$(date +%Y) in .licenserc.yaml..."
	@sed -i -e 's/{{YEAR}}/'"$$\(date +%Y\)"'/g' .licenserc.yaml
	@echo "Year updated in .licenserc.yaml."

license-check-only:
	@echo "Checking license headers..."
	@license-eye header check || { \
		echo; \
		echo "Please add the following header to the top of each Go file:"; \
		echo; \
		echo "// Copyright $$(date +%Y) The OpenChoreo Authors"; \
		echo "// SPDX-License-Identifier: Apache-2.0"; \
		echo; \
		echo "Run \`make license-fix\` locally to fix."; \
		exit 1; \
	}

license-fix-only:
	@echo "Fixing license headers..."
	@license-eye header fix
	@echo "License headers updated."

license-check: install-license-eye update-license-year license-check-only

license-fix: install-license-eye update-license-year license-fix-only


ALL_PKG_DIRS := .
ALL_SRC := $(shell find $(ALL_PKG_DIRS) -type f -name '*.go' \
	! -path './internal/dataplane/kubernetes/types/*' | sort)

checklic:
	@addlicense -c "The OpenChoreo Authors" -s=only .

checklicense:
	@echo "Checking license headers in:"
	@ADDLICENSEOUT=`addlicense -c "The OpenChoreo Authors" -s=only -check $(ALL_SRC) 2>&1`; \
	if [ "$$ADDLICENSEOUT" ]; then \
		echo "❌ License check failed. Errors:"; \
		echo "$$ADDLICENSEOUT"; \
		echo "💡 Use 'make addlicense' to fix this."; \
		exit 1; \
	else \
		echo "✅ License check passed."; \
	fi