SHELL := /bin/bash

.PHONY: bootstrap build check dev clean

bootstrap:
	./scripts/bootstrap-wsl.sh

build:
	PATH="$(CURDIR)/.tools/go/bin:$(CURDIR)/.tools/bin:$$PATH" \
		go build -o .tools/bin/nadesiko-reversi-server ./server

check: build
	PATH="$(CURDIR)/.tools/go/bin:$(CURDIR)/.tools/bin:$$PATH" \
		./scripts/check-wsl.sh

dev: build
	PATH="$(CURDIR)/.tools/go/bin:$(CURDIR)/.tools/bin:$$PATH" \
		.tools/bin/nadesiko-reversi-server -addr 127.0.0.1:4173 -web web -rules rules

clean:
	rm -f .tools/bin/nadesiko-reversi-server

