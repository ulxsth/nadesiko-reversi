SHELL := /bin/bash
TOOL_PATH := $(CURDIR)/.tools/go/bin:$(CURDIR)/.tools/node/bin:$(CURDIR)/.tools/openspec/node_modules/.bin:$(CURDIR)/.tools/bin

.PHONY: bootstrap build check spec-init spec-check dev clean

bootstrap:
	./scripts/bootstrap-wsl.sh

build:
	PATH="$(TOOL_PATH):$$PATH" \
		go build -o .tools/bin/nadesiko-reversi-server ./server

check: build
	PATH="$(TOOL_PATH):$$PATH" \
		./scripts/check-wsl.sh

spec-init:
	PATH="$(TOOL_PATH):$$PATH" openspec init --tools codex --profile core --language Japanese --no-animation

spec-check:
	PATH="$(TOOL_PATH):$$PATH" openspec validate --all --strict

dev: build
	PATH="$(TOOL_PATH):$$PATH" \
		.tools/bin/nadesiko-reversi-server -addr 127.0.0.1:4173 -web web -rules rules

clean:
	rm -f .tools/bin/nadesiko-reversi-server
