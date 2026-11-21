.PHONY: build install test lint fmt clean tidy e2e docs patch minor major check-go

BINARY := terraform-provider-vault-secrets-rancher
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
OS_ARCH := linux_amd64
INSTALL_VERSION := $(shell echo $(VERSION) | sed 's/^v//')
INSTALL_DIR := ~/.terraform.d/plugins/registry.terraform.io/iuclabs/vault-secrets-rancher/$(INSTALL_VERSION)/$(OS_ARCH)

GO_VERSION_REQUIRED := 1.25
GO_VERSION_ACTUAL := $(shell go version | sed 's/go version go\([0-9]*\.[0-9]*\).*/\1/')

check-go:
	@if [ "$$(printf '%s\n%s' "$(GO_VERSION_REQUIRED)" "$(GO_VERSION_ACTUAL)" | sort -V | head -1)" != "$(GO_VERSION_REQUIRED)" ]; then \
		echo "ERROR: Go >= $(GO_VERSION_REQUIRED) required, found $(GO_VERSION_ACTUAL)"; exit 1; \
	fi

build: check-go tidy
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY)

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/

test: check-go
	go test -race -count=1 ./...

lint: check-go
	go vet ./...

docs: check-go
	go generate ./...

fmt: check-go
	gofmt -w .

# End-to-end: boots Vault + mock Rancher + the plugin and applies real terraform.
e2e:
	./scripts/e2e.sh

clean:
	rm -f $(BINARY) *.zip

tidy:
	go mod tidy

DEFAULT_BRANCH ?= main

# Draft the release with generated notes, then push the tag; the release
# workflow attaches assets to the draft and publishes it.
patch minor major:
	@git fetch --quiet --tags origin $(DEFAULT_BRANCH)
	@test "$$(git branch --show-current)" = "$(DEFAULT_BRANCH)" || { echo "ERROR: not on $(DEFAULT_BRANCH)"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "ERROR: working tree not clean"; exit 1; }
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/$(DEFAULT_BRANCH))" || { echo "ERROR: HEAD is not origin/$(DEFAULT_BRANCH)"; exit 1; }
	@LATEST=$$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -1); \
	NEW=$$(echo "$${LATEST:-v0.0.0}" | sed 's/^v//' | awk -F. -v b=$@ '{ \
		if (b == "major") print "v" ($$1+1) ".0.0"; \
		else if (b == "minor") print "v" $$1 "." ($$2+1) ".0"; \
		else print "v" $$1 "." $$2 "." ($$3+1) }'); \
	SHA=$$(git rev-parse HEAD); \
	gh release create "$$NEW" --draft --target "$$SHA" --generate-notes --title "$$NEW" || exit 1; \
	git push origin "$$SHA:refs/tags/$$NEW" || { gh release delete "$$NEW" --yes; exit 1; }
