FROM golang:1.16.5-buster AS builder
WORKDIR /build
COPY . /build/
RUN apt-get update && apt-get install build-essential -y
RUN go build -tags "sqlite_omit_load_extension osusergo netgo" -ldflags="-s -w -linkmode external -extldflags '-fno-PIC -static'" -buildmode=pie -o bin/b-linux-amd64 ./cmd/b

FROM alpine:3.14.0
RUN apk --no-cache add ca-certificates
WORKDIR /app
RUN mkdir /app/data
COPY config.yaml /app/default.yaml
COPY --from=builder /build/bin/b-linux-amd64 /app/b
ENTRYPOINT [ "/app/b" ]
CMD ["-config", "default.yaml"]