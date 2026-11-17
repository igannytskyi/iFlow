#!/bin/sh
# Runs this estate's tests: all of them, or those named — one argument a
# name, each a regular expression matching the name it stands for. It is the
# arbiter every change here is judged by, so a change does not edit it.
cd "$(dirname "$0")" || exit 1
n=$#
for name in "$@"; do set -- "$@" "--test-name-pattern=$name"; done
shift "$n"
exec node --test "$@"
