# syntax = docker/dockerfile:1-experimental

FROM --platform=$BUILDPLATFORM golang:1.19.5-alpine as builder
RUN apk --no-cache add ca-certificates git upx
WORKDIR /app
COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=7 GOAMD64=v3 \
    go build -tags 'osusergo netgo' \
    -ldflags "-s -w -extldflags -static" \
    -o bin/b ./cmd/b
RUN upx --best --lzma bin/b

FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/b /app/b
COPY config.yaml /app/default.yaml

ENTRYPOINT ["/app/b"]
CMD ["-config", "default.yaml"]