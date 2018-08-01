GO ?= go

BUILDTAGS ?= seccomp ostree $(shell hack/selinux_tag.sh) $(shell hack/apparmor_tag.sh)

all: os-container

lint:
	@echo "checking lint"
	@./.tool/lint

gofmt:
	find . -name '*.go' ! -path './vendor/*' -exec gofmt -s -w {} \+
	git diff --exit-code

os-container:
	$(GO) build -i -tags "$(BUILDTAGS)" -o bin/$@ ./cmd/os-container

validate: gofmt

vendor: vendor.conf
	vndr -whitelist '^github.com/containers'
