PACKAGES := github.com/wzshiming/jumpway/app/web/services/...
# gotype v0.8.0 is pinned for generics in Go >=1.18 sources.
GEN := $(abspath $(CURDIR)/../../tmp/gen)

.PHONY: generate
generate: route openapi

$(GEN): $(wildcard ../../tools/gen/go.mod ../../tools/gen/go.sum ../../tools/gen/tools.go)
	mkdir -p $(dir $(GEN)) && go -C ../../tools/gen build -o $(GEN) github.com/wzshiming/gen/cmd/gen

.PHONY: client
client: $(GEN)
	mkdir -p client && $(GEN) client -o ./client/client_gen.go $(PACKAGES)

.PHONY: route
route: $(GEN)
	mkdir -p route && $(GEN) route -o ./route/route_gen.go $(PACKAGES)

.PHONY: openapi
openapi: $(GEN)
	mkdir -p openapi && $(GEN) openapi -o ./openapi/openapi.json $(PACKAGES)

PNPM := pnpm --dir $(CURDIR)/ui

.PHONY: ui
ui:
	$(PNPM) install --frozen-lockfile && $(PNPM) build

.PHONY: ui-check
ui-check:
	$(PNPM) install --frozen-lockfile && $(PNPM) check && $(PNPM) test

.PHONY: ui-dev
ui-dev:
	$(PNPM) install --frozen-lockfile && $(PNPM) dev
