FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/platform-token-runtime ./cmd/platform-token-runtime

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/platform-token-runtime /app/platform-token-runtime
EXPOSE 18081
ENTRYPOINT ["/app/platform-token-runtime"]
