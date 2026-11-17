#!/bin/sh
# Runs this estate's tests: all of them, or those named — one argument a
# name, each a regular expression matching the name it stands for. It is the
# arbiter every change here is judged by, so a change does not edit it.
cd "$(dirname "$0")" || exit 1
set -- $(for n in "$@"; do printf -- '--test-name-pattern=%s\n' "$n"; done)
exec node --test "$@"
