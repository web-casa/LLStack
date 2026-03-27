VERSION ?= 0.1.0-dev
DIST_DIR ?= $(CURDIR)/dist/releases/$(VERSION)
PACKAGE_DIR ?= $(CURDIR)/dist/packages/$(VERSION)
HOST_PLATFORM ?= $(shell go env GOOS)/$(shell go env GOARCH)
PLATFORMS ?= $(HOST_PLATFORM)

.PHONY: test build build-cross package sign-release verify-release validate-release-version release-notes release-summary remote-verify-release release-pipeline publish-release smoke docker-smoke docker-smoke-report install-release clean

test:
	go test ./...

build:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_DIST_DIR="$(DIST_DIR)" \
	LLSTACK_PLATFORMS="$(HOST_PLATFORM)" \
	bash scripts/release/build.sh

build-cross:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_DIST_DIR="$(DIST_DIR)" \
	LLSTACK_PLATFORMS="$(PLATFORMS)" \
	bash scripts/release/build.sh

package: build-cross
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_DIST_DIR="$(DIST_DIR)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	bash scripts/release/package.sh

sign-release:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	LLSTACK_SIGNING_KEY="$(SIGNING_KEY)" \
	LLSTACK_SIGNING_PUBKEY="$(SIGNING_PUBKEY)" \
	bash scripts/release/sign.sh

verify-release:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	bash scripts/release/verify.sh

validate-release-version:
	LLSTACK_VERSION="$(VERSION)" \
	bash scripts/release/validate-version.sh

release-notes:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	LLSTACK_DIST_DIR="$(DIST_DIR)" \
	bash scripts/release/render-notes.sh

release-summary:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	bash scripts/release/post-release-report.sh

publish-release:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	LLSTACK_DIST_DIR="$(DIST_DIR)" \
	bash scripts/release/publish.sh

remote-verify-release:
	LLSTACK_VERSION="$(VERSION)" \
	bash scripts/release/verify-remote.sh

release-pipeline:
	LLSTACK_VERSION="$(VERSION)" \
	LLSTACK_DIST_DIR="$(DIST_DIR)" \
	LLSTACK_PACKAGE_DIR="$(PACKAGE_DIR)" \
	LLSTACK_PLATFORMS="$(PLATFORMS)" \
	bash scripts/release/pipeline.sh $(if $(MODE),$(MODE),validate)

smoke: build
	bash tests/e2e/smoke.sh "$(DIST_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)/llstack"

docker-smoke:
	bash scripts/docker/functional-smoke.sh

docker-smoke-report:
	bash scripts/docker/functional-report.sh

install-release:
	test -n "$(INDEX)" || (echo "INDEX is required, e.g. make install-release INDEX=https://example.invalid/index.json" >&2; exit 1)
	bash scripts/install-release.sh --index "$(INDEX)" $(if $(PLATFORM),--platform "$(PLATFORM)",) $(if $(PREFIX),--prefix "$(PREFIX)",)

clean:
	rm -rf dist
