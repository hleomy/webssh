# 阶段 1: 构建前端
FROM node:20-alpine AS web-builder
WORKDIR /build
COPY web/package.json web/package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com \
 && (npm install --no-audit --no-fund || npm install --no-audit --no-fund)
COPY web/ ./
RUN npm run build

# 阶段 2: 构建 Go 后端
FROM golang:1.22-alpine AS go-builder
WORKDIR /build
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org
ENV CGO_ENABLED=0
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /webssh ./cmd/server

# 阶段 3: 运行时镜像
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata sqlite wget su-exec \
 && addgroup -S webssh -g 1000 && adduser -S webssh -u 1000 -G webssh
WORKDIR /app
COPY --from=go-builder /webssh /app/webssh
COPY --from=web-builder /build/dist /app/web/dist
COPY --chmod=755 deployments/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN mkdir -p /app/data && chown -R webssh:webssh /app
ENV WEBSSH_DATA_DIR=/app/data \
    WEBSSH_SERVER_HOST=0.0.0.0 \
    WEBSSH_SERVER_PORT=6970 \
    TZ=Asia/Shanghai
EXPOSE 6970
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:6970/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/app/webssh"]
