# What is `b`?
[![Go Reference](https://pkg.go.dev/badge/github.com/zllovesuki/b.svg)](https://pkg.go.dev/github.com/zllovesuki/b) ![testing](https://github.com/zllovesuki/b/actions/workflows/test.yaml/badge.svg) [![codecov](https://codecov.io/gh/zllovesuki/b/branch/main/graph/badge.svg?token=LJHGK83MNI)](https://codecov.io/gh/zllovesuki/b)

I got bored during 4th of July weekend, so a mini weekend hackathon to put something together.

In short:

1. file hosting, and
2. link shortening, and
3. pastebin

Project is inspired by [https://github.com/raftario/filite](https://github.com/raftario/filite). Actually, `index.html` is adopted from `filite`, because I'm too lazy to write a frontend.

# Configuration

Currently the MVP is hard coded to use:

1. file hosting: metadata on redis, actual storage on disk
2. link shortening: redis
3. text sharing: redis

And the baseURL is also hard coded to use `http://127.0.0.1:3000`

In a future version it is planned to add:

1. S3/S3-compatible storage to back file hosting
2. *SQL and its garden varieties for link/text/file metadata
3. Anything you feel like you want to add. The interface exists in `app/backend.go`
4. Environmental variables based configurations

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
