#!/usr/bin/env sh

set -eu
set -o pipefail

scheme="${GIT_CREDENTIAL_URL%%://*}"
host="${GIT_CREDENTIAL_URL#*://}"

case "$1" in
    "Username for '$scheme://$host': ") exec echo "$GIT_USERNAME" ;;
    "Password for '$scheme://$GIT_USERNAME@$host': ") exec echo "$GIT_PASSWORD" ;;
esac
exit 1
