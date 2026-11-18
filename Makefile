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

_LATEST := $(shell git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -1)
_BASE   := $(if $(_LATEST),$(_LATEST),v0.0.0)
_MAJ    := $(shell echo $(_BASE) | sed 's/^v//' | cut -d. -f1)
_MIN    := $(shell echo $(_BASE) | sed 's/^v//' | cut -d. -f2)
_PAT    := $(shell echo $(_BASE) | sed 's/^v//' | cut -d. -f3)

patch:
	@NEW=v$(_MAJ).$(_MIN).$(shell expr $(_PAT) + 1); \
	git tag $$NEW && echo "Tagged $$NEW" && git push origin $$NEW

minor:
	@NEW=v$(_MAJ).$(shell expr $(_MIN) + 1).0; \
	git tag $$NEW && echo "Tagged $$NEW" && git push origin $$NEW

major:
	@NEW=v$(shell expr $(_MAJ) + 1).0.0; \
	git tag $$NEW && echo "Tagged $$NEW" && git push origin $$NEW
