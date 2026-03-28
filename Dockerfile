FROM node:20-bookworm AS web-build
WORKDIR /workspace/web

COPY web/package*.json ./
RUN npm install

COPY web/ ./
RUN npm run build

FROM golang:1.21-bookworm AS go-build
WORKDIR /workspace

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/ ./web/
COPY --from=web-build /workspace/web/dist ./web/dist

RUN go build -o /out/hearthstone-analyzer ./cmd/api

FROM debian:bookworm-slim
WORKDIR /app

ENV APP_ADDR=:8080
ENV APP_DATA_DIR=/data

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /data

COPY --from=go-build /out/hearthstone-analyzer /app/hearthstone-analyzer

VOLUME ["/data"]

EXPOSE 8080

CMD ["/app/hearthstone-analyzer"]
