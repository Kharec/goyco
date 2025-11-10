ARG GO_VERSION=1.25.3

# Building the binary using a golang alpine image
FROM golang:${GO_VERSION}-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/goyco ./cmd/goyco

# building the application image
FROM alpine:3.21
RUN addgroup -S goyco && adduser -S -G goyco goyco \
  && apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /out/goyco ./goyco
COPY --from=go-builder /src/internal/static ./internal/static
COPY --from=go-builder /src/internal/templates ./internal/templates
RUN mkdir -p /app/log /app/run && chown -R goyco:goyco /app
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080
EXPOSE 8080
USER goyco

ENTRYPOINT ["./goyco", "run"]
