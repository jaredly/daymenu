#!/bin/bash
set -ex
go build
cp menunder Menunder.app/Contents/MacOS
cp menunder /Applications/Menunder.app/Contents/MacOS