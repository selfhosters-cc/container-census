#!/bin/bash
CGO_ENABLED=1 go build -o container-census cmd/server/main.go