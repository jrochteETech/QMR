FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go schema.sql ./
COPY Web ./Web
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/qmr .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && addgroup -S qmr && adduser -S -G qmr qmr
WORKDIR /app
COPY --from=build /out/qmr /app/qmr
USER qmr
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/qmr"]