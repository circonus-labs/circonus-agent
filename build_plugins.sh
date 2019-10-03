#!/usr/bin/env bash

echo
echo "Building io_latency plugin for linux"
echo
pushd plugins/linux/io/io_latency 
GOOS=linux GOARCH=amd64 go build -v -o io_latency.elf || exit 1
popd
