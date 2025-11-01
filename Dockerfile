FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server

FROM gcr.io/distroless/base-debian12

WORKDIR /app

ENV PORT=8080

COPY --from=builder /app/server .

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/server"]