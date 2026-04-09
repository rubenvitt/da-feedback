FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o da-feedback ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/da-feedback .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static
EXPOSE 8080
ENTRYPOINT ["./da-feedback"]
