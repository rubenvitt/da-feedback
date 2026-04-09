FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o da-feedback ./cmd/server

FROM debian:bookworm-slim AS css
WORKDIR /build
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && \
    ARCH=$(dpkg --print-architecture) && \
    if [ "$ARCH" = "amd64" ]; then TW_ARCH="x64"; else TW_ARCH="arm64"; fi && \
    curl -sLO "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-${TW_ARCH}" && \
    chmod +x tailwindcss-linux-${TW_ARCH} && mv tailwindcss-linux-${TW_ARCH} tailwindcss
COPY static ./static
COPY templates ./templates
RUN ./tailwindcss -i static/input.css -o static/style.css --minify

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/da-feedback .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static
COPY --from=css /build/static/style.css ./static/style.css
EXPOSE 8080
ENTRYPOINT ["./da-feedback"]
