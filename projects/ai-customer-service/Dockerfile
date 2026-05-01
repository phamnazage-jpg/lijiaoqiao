FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ai-cs ./cmd/ai-customer-service

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/ai-cs /ai-cs
EXPOSE 8080
ENTRYPOINT ["/ai-cs"]
