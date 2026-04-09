FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o da-feedback ./cmd/server

FROM node:lts-alpine AS css
WORKDIR /build
RUN npm install tailwindcss @tailwindcss/cli
COPY static ./static
COPY templates ./templates
RUN npx @tailwindcss/cli -i static/input.css -o static/style.css --minify

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/da-feedback .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static
COPY --from=css /build/static/style.css ./static/style.css
EXPOSE 8080
ENTRYPOINT ["./da-feedback"]
