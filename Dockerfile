FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o rest-server ./cmd/rest-server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/rest-server .

EXPOSE 8080
CMD ["./rest-server"]