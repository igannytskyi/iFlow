#!/bin/sh
# Runs this estate's tests, the named ones or all of them. It is the arbiter
# every change here is judged by, so a change does not edit it.
cd "$(dirname "$0")" || exit 1
if [ -n "$1" ]; then exec python3 -m unittest -k "$1" -v; fi
exec python3 -m unittest -v
