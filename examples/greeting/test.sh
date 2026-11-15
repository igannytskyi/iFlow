#!/bin/sh
# Runs this estate's tests, the named ones or all of them. It is the arbiter
# every change here is judged by, so a change does not edit it.
cd "$(dirname "$0")" && exec go test -count=1 -run "${1:-.}" .
