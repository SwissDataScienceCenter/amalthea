#!/bin/bash
#{{SBATCH_DIRECTIVES}}

set -e -o pipefail

GIT_PROXY_WAIT_SLEEP_SECONDS=10
GIT_PROXY_WAIT_RETRIES=10

# Installs rclone
#
# Usage:
#     rclone="$(install_rclone)"
#     "$rclone" version
function install_rclone() {
    RENKU_DIR="${HOME}/.renku/$(uname -m)"
    RENKU_PKG="${RENKU_DIR}/pkg"
    RCLONE_VERSION="1.70.2"
    RCLONE_PKG="${RENKU_PKG}/rclone/v${RCLONE_VERSION}"
    RCLONE_BIN="${RCLONE_PKG}/rclone"

    skip_install="0"
    if [ -f "${RCLONE_BIN}" ]; then
        version="$("${RCLONE_BIN}" version || echo "bad executable")"
        version="$(echo "${version}" | head -n 1)"
        expected="rclone v${RCLONE_VERSION}"
        if [ "${version}" = "${expected}" ]; then
            skip_install="1"
        else
            >&2 echo "WARNING: found mismatching rclone version ${version}"
        fi
    fi

    if [ "${skip_install}" != "0" ]; then
        echo "${RCLONE_BIN}"
        return 0
    fi

    arch="$(uname -m)"
    if [ "${arch}" = "x86_64" ]; then
        RCLONE_URL="https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.zip"
    elif [ "${arch}" = "aarch64" ]; then
        RCLONE_URL="https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-arm64.zip"
    else
        >&2 echo "Unsupported platform: ${arch}"
        exit 1
    fi

    mkdir -p "${RCLONE_PKG}"
    tmp="$(mktemp -d)"
    cwd="$(pwd)"
    cd "${tmp}"
    curl -Lo "rclone.zip" "${RCLONE_URL}"
    >&2 unzip "rclone.zip"
    rm -r "${RCLONE_PKG}"
    mv ./rclone-v"${RCLONE_VERSION}"-* "${RCLONE_PKG}"
    rm -r "${tmp}"
    chmod a+x "${RCLONE_BIN}"

    echo "${RCLONE_BIN}"
}

# Installs wstunnel
#
# Usage:
#     wstunnel="$(install_wstunnel)"
#     "$wstunnel" --version
function install_wstunnel() {
    RENKU_DIR="${HOME}/.renku/$(uname -m)"
    RENKU_PKG="${RENKU_DIR}/pkg"
    WSTUNNEL_VERSION="10.4.4"
    WSTUNNEL_PKG="${RENKU_PKG}/wstunnel/v${WSTUNNEL_VERSION}"
    WSTUNNEL_BIN="${WSTUNNEL_PKG}/wstunnel"

    arch="$(uname -m)"
    if [ "${arch}" = "aarch64" ]; then
        WSTUNNEL_VERSION_FORCED="10.1.10"
        >&2 echo "Warning: using wstunnel v${WSTUNNEL_VERSION_FORCED} instead of ${WSTUNNEL_VERSION}"
        WSTUNNEL_VERSION="${WSTUNNEL_VERSION_FORCED}"
    fi

    skip_install="0"
    if [ -f "${WSTUNNEL_BIN}" ]; then
        version="$("${WSTUNNEL_BIN}" --version || echo "bad executable")"
        expected="wstunnel-cli ${WSTUNNEL_VERSION}"
        if [ "${version}" = "${expected}" ]; then
            skip_install="1"
        else
            >&2 echo "WARNING: found mismatching wstunnel version ${version}"
        fi
    fi

    if [ "${skip_install}" != "0" ]; then
        echo "${WSTUNNEL_BIN}"
        return 0
    fi

    arch="$(uname -m)"
    if [ "${arch}" = "x86_64" ]; then
        WSTUNNEL_URL="https://github.com/erebe/wstunnel/releases/download/v${WSTUNNEL_VERSION}/wstunnel_${WSTUNNEL_VERSION}_linux_amd64.tar.gz"
    elif [ "${arch}" = "aarch64" ]; then
        WSTUNNEL_URL="https://github.com/erebe/wstunnel/releases/download/v${WSTUNNEL_VERSION}/wstunnel_${WSTUNNEL_VERSION}_linux_arm64.tar.gz"
    else
        >&2 echo "Unsupported platform: ${arch}"
        exit 1
    fi

    mkdir -p "${WSTUNNEL_PKG}"
    tmp="$(mktemp -d)"
    cwd="$(pwd)"
    cd "${tmp}"
    curl -Lo "wstunnel.tar.gz" "${WSTUNNEL_URL}"
    tar xf "wstunnel.tar.gz" -C "${WSTUNNEL_PKG}"
    cd "${cwd}"
    rm -r "${tmp}"
    chmod a+x "${WSTUNNEL_BIN}"

    echo "${WSTUNNEL_BIN}"
}

if [ -z "${REMOTE_SESSION_IMAGE}" ]; then
    echo "REMOTE_SESSION_IMAGE is not set, aborting!"
    exit 1
fi

SESSION_DIR="$(pwd)"
SESSION_WORK_DIR="${SESSION_DIR}/work"
SECRETS_DIR="${SESSION_DIR}/secrets"
LOGS_DIR="${SESSION_DIR}/logs"
echo "SESSION_DIR: ${SESSION_DIR}"
echo "SESSION_WORK_DIR: ${SESSION_WORK_DIR}"

mkdir -p "${SESSION_WORK_DIR}"
mkdir -p "${SECRETS_DIR}"
mkdir -p "${LOGS_DIR}"

# # Install rclone
# rclone=$(install_rclone)
# echo "rclone: ${rclone}"

# Install wstunnel
wstunnel=$(install_wstunnel)
echo "wstunnel: ${wstunnel}"

# Ensure NVIDIA_VISIBLE_DEVICES is set to void 
# so that cuda enabled images work on eiger
if !(nvidia-smi 2>&1 >/dev/null); then
    export NVIDIA_VISIBLE_DEVICES=void
fi

# Create the environment.toml file to run the session
EDF_FILE="${SESSION_DIR}/environment.toml"
cat <<EOF >"${EDF_FILE}"
image = "${REMOTE_SESSION_IMAGE}"

# mounts = [
#     "${SCRATCH}",
#     "${SECRETS_DIR}:/secrets:ro",
# ]
#{{SESSION_MOUNTS}}

workdir = "${SESSION_WORK_DIR}"

[annotations]
com.hooks.cxi.enabled = "false"
EOF

export RENKU_MOUNT_DIR="${SESSION_WORK_DIR}"
export RENKU_WORKING_DIR="${SESSION_WORK_DIR}"
# Force the frontend to listen on 127.0.0.1
export RENKU_SESSION_IP="127.0.0.1"

# Load the wstunnel secret
export WSTUNNEL_SECRET="$(cat "${SECRETS_DIR}/wstunnel_secret")"

echo "TODO: setup rclone mounts..."

# echo "Setting up example rclone mount..."
# fusermount3 -u "${SESSION_WORK_DIR}/era5" || true
# rm -rf "${SESSION_WORK_DIR}/era5"
# mkdir -p "${SESSION_WORK_DIR}/era5"
# RCLONE_CONFIG="${SESSION_DIR}/rclone.conf"
# cat <<EOF >"${RCLONE_CONFIG}"
# [era5]
# type = doi
# doi = 10.5281/zenodo.3831980
# EOF
# "${rclone}" mount --config "${RCLONE_CONFIG}" --daemon --read-only era5: "${SESSION_WORK_DIR}/era5"

# echo "Starting tunnel..."
GIT_PROXY_PORT="${GIT_PROXY_PORT:-65480}"
GIT_PROXY_HEALTH_PORT="${GIT_PROXY_HEALTH_PORT:-65481}"
WSTUNNEL_PATH_PREFIX="${WSTUNNEL_PATH_PREFIX:-sessions/my-session/wstunnel}"
echo "wstunnel client \
  -R tcp://0.0.0.0:${RENKU_SESSION_PORT}:localhost:${RENKU_SESSION_PORT} \
  -L tcp://${GIT_PROXY_PORT}:localhost:${GIT_PROXY_PORT} \
  -L tcp://${GIT_PROXY_HEALTH_PORT}:localhost:${GIT_PROXY_HEALTH_PORT} \
  wss://${WSTUNNEL_SERVICE_ADDRESS}:${WSTUNNEL_SERVICE_PORT} \
  -P ${WSTUNNEL_PATH_PREFIX} \
  -H Authorization: Bearer <SECRET> \
  --tls-verify-certificate &"
"${wstunnel}" client \
  -R "tcp://0.0.0.0:${RENKU_SESSION_PORT}:localhost:${RENKU_SESSION_PORT}" \
  -L tcp://${GIT_PROXY_PORT}:localhost:${GIT_PROXY_PORT} \
  -L tcp://${GIT_PROXY_HEALTH_PORT}:localhost:${GIT_PROXY_HEALTH_PORT} \
  "wss://${WSTUNNEL_SERVICE_ADDRESS}:${WSTUNNEL_SERVICE_PORT}" \
  -P "${WSTUNNEL_PATH_PREFIX}" \
  -H "Authorization: Bearer ${WSTUNNEL_SECRET}" \
  --tls-verify-certificate 2>&1 >"${LOGS_DIR}/wstunnel.logs" &

if [ -n "${GIT_REPOSITORIES}" ]; then
    echo "Waiting for git proxy..."
    git_proxy_ready="0"
    for i in $(seq 1 "${GIT_PROXY_WAIT_RETRIES}"); do
        set +e
        curl -sSL --fail -o /dev/null "http://localhost:${GIT_PROXY_HEALTH_PORT}/health" 2>/dev/null
        ready="$(echo $?)"
        set -e
        if [ "${ready}" == "0" ]; then
            git_proxy_ready="1"
            break
        fi
        echo "Git proxy not ready ${i}/${GIT_PROXY_WAIT_RETRIES}..."
        sleep "${GIT_PROXY_WAIT_SLEEP_SECONDS}"
    done
    if [ "${git_proxy_ready}" == "0" ]; then
        echo "Git proxy not ready, aborting"
        exit 1
    fi

    echo "Setting up git repositories..."
    cwd="$(pwd)"
    OIFS="${IFS}"
    IFS=$'\n'
    GIT_REPOSITORIES=(${GIT_REPOSITORIES})
    IFS="${OIFS}"
    for line in "${GIT_REPOSITORIES[@]}"; do
        repo="$(echo "${line}" | cut -d$'\t' -f1)"
        branch="$(echo "${line}" | cut -d$'\t' -f2)"
        echo "repo: ${repo}, branch: ${branch}"
        cd "${RENKU_WORKING_DIR}/${repo}"
        git init
        git fetch
        if [ -n "${branch}" ]; then
            git checkout "${branch}"
            git pull
        fi
    done
    cd "${cwd}"
fi

exit_script() {
    echo "Cleaning up session..."
    # fusermount3 -u "${SESSION_WORK_DIR}/era5" || true
}

echo "Starting session..."
# Start session while listening to EXIT signals
pid=
trap 'exit_script && [[ $pid ]] && kill -TERM "$pid" && exit_script' EXIT
srun --environment "${EDF_FILE}" --no-container-entrypoint sh /etc/rc & pid=$!
wait
pid=
