.PHONY: build test generate publish install-wgc install-tools

# Inherit the pinned protoc 29.3 from the router project (tools/protoc-29.3)
# — wgc rejects any other protoc major. No manual PATH export needed.
TOOLS_DIR := $(CURDIR)/../../tools/protoc-29.3
WGC := PATH=$(TOOLS_DIR)/bin:$(PATH):$(HOME)/go/bin

install-tools:
	$(MAKE) -C ../.. install-tools

install-wgc:
	@which wgc > /dev/null 2>&1 || npm install -g wgc@latest

make: build

test: install-tools install-wgc
	$(WGC) wgc router plugin test .

generate: install-tools install-wgc
	$(WGC) wgc router plugin generate .

publish: generate
	$(WGC) wgc router plugin publish .

build: install-tools install-wgc
	$(WGC) wgc router plugin build . --debug
