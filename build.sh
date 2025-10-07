#!/bin/bash
set -ex
go build -ldflags="-X main.Version=$(git rev-parse HEAD)"
cp menunder Menunder.app/Contents/MacOS
cp menunder /Applications/Menunder.app/Contents/MacOS