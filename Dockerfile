# ==========================================
# 第一阶段：构建 (Builder)
# ==========================================
FROM golang:1.25-alpine AS builder

WORKDIR /src

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/qqqai .

# ==========================================
# 第二阶段：运行 (Runner)
# ==========================================
FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata nodejs npm \
    && npm install --omit=dev --no-audit --no-fund mcp-server-mysql \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/qqqai ./qqqai

RUN mkdir -p /app/data/memory /app/tmp \
    && chown -R app:app /app

USER app

EXPOSE 8080

CMD ["./qqqai"]
