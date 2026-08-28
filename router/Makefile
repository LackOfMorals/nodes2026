.PHONY: install-wgc install-tools build download start compose render-graph neo4j-api

# wgc's toolchain check is strict about protoc (must be 29-series; Homebrew
# ships 36.0 and is rejected). Keep a pinned protoc in tools/ and put it
# FIRST on PATH for every wgc invocation — no manual export needed.
TOOLS_DIR := $(CURDIR)/tools/protoc-29.3
WGC := PATH=$(TOOLS_DIR)/bin:$(PATH):$(HOME)/go/bin

install-tools:
	@if [ ! -x $(TOOLS_DIR)/bin/protoc ]; then \
		echo "Fetching protoc 29.3 (wgc requires the 29-series)..."; \
		curl -fsSL -o /tmp/protoc-29.3-osx-aarch_64.zip \
		  https://github.com/protocolbuffers/protobuf/releases/download/v29.3/protoc-29.3-osx-aarch_64.zip; \
		unzip -oq /tmp/protoc-29.3-osx-aarch_64.zip -d $(TOOLS_DIR); \
		rm -f /tmp/protoc-29.3-osx-aarch_64.zip; \
	fi

install-wgc:
	@which wgc > /dev/null 2>&1 || npm install -g wgc@latest

# Render the gitignored graph.yaml from the committed template, inlining
# the Linear API key from the repo-root .env (wgc 0.130.1 does no env
# interpolation in graph.yaml).
render-graph:
	@test -f ../.env || { echo "error: ../.env not found"; exit 1; }
	@key=$$(grep '^LINEAR_API_KEY=' ../.env | cut -d= -f2-); \
	test -n "$$key" || { echo "error: LINEAR_API_KEY missing from ../.env"; exit 1; }; \
	sed "s|__LINEAR_API_KEY__|$$key|" graph.yaml.template > graph.yaml

# One-command start: router binary + Slack plugin + rendered graph +
# composed config, then the router in the foreground on :3010.
start: download build compose
	@set -a; . ../.env; set +a; exec ./release/router

# Run the local Neo4j subgraph service in the foreground (POST :4400/graphql).
neo4j-api:
	@set -a; . ../.env; set +a; cd ../neo4j-api \
		&& go build -o bin/darwin_arm64 . && exec ./bin/darwin_arm64

compose: render-graph install-tools install-wgc
	$(WGC) wgc router compose -i graph.yaml -o config.json

download: install-wgc
	@if [ ! -f release/router ]; then \
		rm -rf release && wgc router download-binary -o release && chmod +x release/router; \
	else \
		echo "Router binary already exists, skipping download"; \
	fi

build:
	cd plugins/slack && make build
