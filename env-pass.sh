#!/usr/bin/env sh

set -eu
set -o pipefail

# Redirect to stderr so as to not interfere with the stdout needed for username/password prompts
printenv >&2 # Do not try echo "$GIT_PREFIX", it doesn't seem to be inherited by this process

case "$1" in
    Username*) exec echo "$GIT_USERNAME" ;;
    Password*) exec echo "$GIT_PASSWORD" ;;
esac
