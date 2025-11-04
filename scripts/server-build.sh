#!/bin/bash
CGO_ENABLED=1 go build -o /tmp/census-server ./cmd/server && ls -lh /tmp/census-server
