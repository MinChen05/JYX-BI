SERVER := apps/server
WEB := apps/web

.PHONY: test build dev docker tsc validate-tpl

test:
	cd $(SERVER) && go test ./...

validate-tpl:
	cd $(SERVER) && go run ./cmd/admin validate-tpl -dir ../../templates

tsc:
	cd $(WEB) && npm run tsc

dev:
	cd $(SERVER) && go run ./cmd/server -config config.ini

build:
	cd $(WEB) && npm ci && npm run build
	cd $(SERVER) && CGO_ENABLED=0 go build -trimpath \
		-ldflags "-s -w -X main.version=$$(git describe --tags --always 2>/dev/null || echo dev)" \
		-o bin/kingdee-rpt ./cmd/server

docker:
	docker build -f deploy/Dockerfile -t kingdee-rpt:dev .
