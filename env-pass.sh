#!/usr/bin/env sh

set -eu
set -o pipefail

case "$1" in
    Username*) exec echo "$GIT_USERNAME" ;;
    Password*) exec echo "$GIT_PASSWORD" ;;
esac
