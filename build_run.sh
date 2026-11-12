#!/usr/bin/sh

set -xe

rm ./dice

go build -ldflags="-s -w" -trimpath -o dice ./cmd/server

./dice
