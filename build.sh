#!/usr/bin/sh

set -xe

go build -ldflags="-s -w" -trimpath -o dice ./cmd/server
