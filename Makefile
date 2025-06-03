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
	@echo "✅ Year updated in .licenserc.yaml"

license-fix: install-license-eye update-license-year
	@echo "Fixing license headers..."
	@license-eye header fix
	@echo "✅ License headers updated."

license-check: install-license-eye update-license-year
	@echo "Checking license headers..."
	@license-eye header check || { \
		echo "/n"; \
		echo "❌ License header check failed."; \
		echo "👉 Run 'make license-fix' to automatically fix headers."; \
		exit 1; \
	}