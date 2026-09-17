VERSION ?= 0.3.0
GO ?= go
GOARCH ?= amd64
PLUGIN_ID = cpa-pool-supervise
OUT = dist/cpa-pool-supervise-v$(VERSION)-linux-$(GOARCH).so
ARCHIVE = dist/$(PLUGIN_ID)_$(VERSION)_linux_$(GOARCH).zip
ARCHIVE_CHECKSUM = $(ARCHIVE).sha256
DEPLOY_DIR ?= ../../plugins/linux/$(GOARCH)

.PHONY: build package test deploy clean

build:
	test "$$($(GO) env GOOS)" = "linux"
	test "$$($(GO) env GOARCH)" = "amd64"
	mkdir -p "$(dir $(OUT))"
	CGO_ENABLED=1 $(GO) build -buildvcs=false -trimpath -buildmode=c-shared \
		-ldflags "-s -w -X main.pluginVersion=$(VERSION)" -o "$(OUT)" ./src
	rm -f dist/*.h

package: build
	$(GO) run -buildvcs=false ./.github/scripts/package-release.go \
			-library "$(OUT)" -entry "$(PLUGIN_ID).so" \
			-archive "$(ARCHIVE)" -checksum "$(ARCHIVE_CHECKSUM)"
	cp "$(ARCHIVE_CHECKSUM)" dist/checksums.txt

test:
	$(GO) vet ./...
	$(GO) test ./...

deploy: build
	mkdir -p "$(DEPLOY_DIR)"
	cp "$(OUT)" "$(DEPLOY_DIR)/"
	docker restart cli-proxy-api

clean:
	rm -rf dist
