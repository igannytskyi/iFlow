#!/bin/sh
# Runs this estate's tests, the named ones or all of them. It is the arbiter
# every change here is judged by, so a change does not edit it.
cd "$(dirname "$0")" || exit 1
if [ -n "$1" ]; then exec node --test --test-name-pattern="$1"; fi
exec node --test
