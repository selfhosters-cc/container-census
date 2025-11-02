#!/bin/bash
# CGO_ENABLED=1 go build -o container-census cmd/server/main.go

CGO_ENABLED=1 go build -o /tmp/container-census ./cmd/server && ls -lh /tmp/container-census
