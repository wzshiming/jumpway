PACKAGES := github.com/wzshiming/jumpway/app/web/services/...

.PHONY: generate
generate: route openapi

.PHONY: client
client:
	mkdir -p client && gen client -o ./client/client_gen.go ${PACKAGES}

.PHONY: route
route:
	mkdir -p route && gen route -o ./route/route_gen.go ${PACKAGES}

.PHONY: openapi
openapi:
	mkdir -p openapi && gen openapi -o ./openapi/openapi.json ${PACKAGES}