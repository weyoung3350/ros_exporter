FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY mcp-server.go .
COPY go.mod go.sum ./
RUN go mod tidy && go build -o mcp-server mcp-server.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/mcp-server .

# 创建数据目录
RUN mkdir -p /data/dashboards

EXPOSE 8080

CMD ["./mcp-server"] 