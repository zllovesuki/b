# What is `b`?
[![Go Reference](https://pkg.go.dev/badge/github.com/zllovesuki/b.svg)](https://pkg.go.dev/github.com/zllovesuki/b) ![Pipeline](https://github.com/zllovesuki/b/actions/workflows/pipeline.yaml/badge.svg) [![codecov](https://codecov.io/gh/zllovesuki/b/branch/main/graph/badge.svg?token=LJHGK83MNI)](https://codecov.io/gh/zllovesuki/b)

I got bored during 4th of July weekend, so a mini weekend hackathon to put something together.

In short:

1. file hosting, and
2. link shortening, and
3. pastebin

Project is inspired by [https://github.com/raftario/filite](https://github.com/raftario/filite). Actually, `index.html` is adopted from `filite`, because I'm too lazy to write a frontend.

# Use cases

## Quick file exchange with friends

If you want to exchange some files (say 50MB of photos from last week's trip) with a friend, one of you can use [TryCloudflare](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/trycloudflare#using-trycloudflare), then run `b` with baseURL set to your tunnel, so the other person can quickly upload/download file.

## All-in-One self-hosted solutions

If you want pastebin, bit.ly, and Firefox Send (roughly) all in the same spot, then `b` is perfect for you. You can even setup access control so `POST` requests can only come from your VPN, or requiring a password, so you can leave `b` running openly on the internet.

## Promotion

Sometimes you just want to show off the shortest domain you own and let everyone knows 
😂.

# Configuration

Please see `config.yaml` for reference.

# How to run

`./bin/b.exe -config config.yaml`

The default `config.yaml` should be sufficient for most use cases. Visit `http://127.0.0.1:3000` to use the web interface.

# Command Line usage

Uploading a file:
```bash
# Specifying an expiration time is in TODO.

curl -X PUT -F file=@Alaska.jpg https://example.com:3000/f-alaskan
{"result":"https://example.com:3000/f-alaskan","error":null,"messages":[]}
```

Pasting some text:
```bash
# Optionally, you can specify when the paste expires in seconds: https://example.com:3000/t-footxt/60

cat foo.txt | curl -X PUT --data-binary @- https://example.com:3000/t-footxt
{"result":"https://example.com:3000/t-footxt","error":null,"messages":[]}
```

Shortening a link:
```bash
# Optionally, you can specify when the link expires in seconds: https://example.com:3000/l-longurl/60

curl -H "Content-Type: application/json" \
    -X PUT \
    --data '{"url": "https://llanfairpwllgwyngyllgogerychwyrndrobwllllantysiliogogogoch.co.uk/"}' \
    https://example.com:3000/l-longurl
{"result":"https://example.com:3000/l-longurl","error":null,"messages":[]}
```

# TODO

In a future version it is planned to add:

1. ~~S3/S3-compatible storage to back file hosting~~ (done!)
2. ~~*SQL and its garden varieties for link/text/file metadata~~ (added SQLite for `app.Backend`, not `app.FastBackend` though)
3. ~~Environmental variables based configurations~~ (done via `config.yaml`)
4. Access control
5. TTL for file service
6. Anything you feel like you want to add. The interface exists in `app/backend.go`

# How to develop locally

Requirements:
- Golang 1.16.5
- Docker & Docker-Compose
- Your favorite IDE

1. First, run `docker-compose up -d`
2. Then, ensure that the tests pass with `go test -v -race ./...`

Disclaimer: I mainly test on Windows, and Github Actions test on ubuntu. macOS is currently untested.

# How to build

`go build -tags sqlite_omit_load_extension,osusergo,netgo -ldflags="-s -w -linkmode external -extldflags -static" -o bin/b.exe ./cmd/b/`

This will build `b` in `bin/b.exe` with stripped debug info, and statically without dynamically linked libs.

For building on macOS host (amd64/arm64), remove `-extldflags -static`.
