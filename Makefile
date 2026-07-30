GO ?= go
BINARY := openwrt2mqtt

.PHONY: build test fmt clean

build:
	$(GO) build -trimpath -o build/$(BINARY) ./cmd/$(BINARY)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf build