FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /memorix-server ./cmd/memorix-server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /memorix-server /memorix-server
EXPOSE 8080
ENTRYPOINT ["/memorix-server"]
