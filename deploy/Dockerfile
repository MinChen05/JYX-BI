# 多阶段构建：前端 dist 由 Go 二进制直接服务（单二进制部署）
FROM node:22-alpine AS web
WORKDIR /build/web
COPY apps/web/package.json apps/web/.npmrc ./
RUN npm install --no-audit --no-fund
COPY apps/web/ ./
COPY packages/ /build/packages/
RUN npm run build

FROM golang:1.25-alpine AS server
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build/server
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download
COPY apps/server/ ./
COPY templates/ /build/templates/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/kingdee-rpt ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
WORKDIR /app
COPY --from=server /out/kingdee-rpt /app/kingdee-rpt
COPY --from=web /build/web/dist /app/web-dist
COPY templates/ /app/templates
EXPOSE 8090
ENTRYPOINT ["/app/kingdee-rpt", "-config", "/app/config.ini"]
