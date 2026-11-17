#!/bin/sh
# Runs this estate's tests: all of them, or those named — one argument a
# name, each a regular expression matching the name it stands for. It is the
# arbiter every change here is judged by, so a change does not edit it.
cd "$(dirname "$0")" || exit 1
if [ $# -eq 0 ]; then exec go test -count=1 ./...; fi
pattern=$(printf '%s|' "$@")
exec go test -count=1 -v -run "^(${pattern%|})\$" ./...
