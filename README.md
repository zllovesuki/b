# What is `b`?
[![Go Reference](https://pkg.go.dev/badge/github.com/zllovesuki/b.svg)](https://pkg.go.dev/github.com/zllovesuki/b) ![testing](https://github.com/zllovesuki/b/actions/workflows/test.yaml/badge.svg) [![codecov](https://codecov.io/gh/zllovesuki/b/branch/main/graph/badge.svg?token=LJHGK83MNI)](https://codecov.io/gh/zllovesuki/b)

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

Currently the MVP is hard coded to use:

1. file hosting: metadata on redis, actual storage on disk
2. link shortening: redis
3. text sharing: redis

And the baseURL is also hard coded to use `http://127.0.0.1:3000`

In a future version it is planned to add:

1. ~~S3/S3-compatible storage to back file hosting~~ (done!)
2. ~~*SQL and its garden varieties for link/text/file metadata~~ (added SQLite for `app.Backend`, not `app.FastBackend` though)
3. ~~Environmental variables based configurations~~ (done via `config.yaml`)
4. Access control
5. Anything you feel like you want to add. The interface exists in `app/backend.go`

# How to develop locally

Requirements:
- Golang 1.16.5
- Docker & Docker-Compose
- Your favorite IDE

1. First, run `docker-compose up -d`
2. Then, ensure that the tests pass with `go test -v -race ./...`

Disclaimer: I mainly test on Windows, and Github Actions test on ubuntu. macOS is currently untested.

# How to build

`go build -tags osusergo,netgo -ldflags="-extldflags=-static -s -w" -o bin/b.exe ./cmd/b/`

This will build `b` in `bin/b.exe` with stripped debug info, and statically without dynamically linked libs.
