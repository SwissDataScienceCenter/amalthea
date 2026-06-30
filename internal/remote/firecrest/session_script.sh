#!/bin/bash
# NOTE FOR AMALTHEA MAINTAINERS:
#   This script contains template strings in the following form:
#     `#{{NAME}}`
#   These strings should be added or removed according to code changes
#   in the remote session controller.
# END NOTE
#{{SBATCH_DIRECTIVES_PLACEHOLDER}}

set -e -o pipefail

: ${REMOTE_SESSION_IMAGE:?'not set, aborting!'}

: ${ARCH:=$(uname -m)}
: ${RENKU_PKG:="${HOME}/.renku/${ARCH}/pkg"}
: ${GIT_PROXY_PORT:=65480}
: ${GIT_PROXY_HEALTH_PORT:=65481}
: ${GIT_PROXY_WAIT_SLEEP_SECONDS:=10}
: ${GIT_PROXY_WAIT_RETRIES:=10}
: ${RCLONE_VERSION:="1.70.2"}
: ${WSTUNNEL_PATH_PREFIX:="sessions/my-session/wstunnel"}
: ${WSTUNNEL_VERSION:="10.5.5"}

case ${ARCH} in
    "x86_64")
        gh_arch=amd64
        ;;
    "aarch64")
        gh_arch=arm64
        ;;
    *)
        >&2 echo "Unsupported platform: ${ARCH}"
        exit 1
        ;;
esac

: ${SESSION_DIR:="${PWD}"}
: ${SESSION_WORK_DIR:="${SESSION_DIR}/work"}
: ${SECRETS_DIR:="$(mktemp -d)"}
: ${SECRETS_USER_DIR:="${SECRETS_DIR}/user/"}
: ${SECRETS_DATA_CONNECTORS_DIR:="${SECRETS_DIR}/data_connectors"}
: ${LOGS_DIR:="${SESSION_DIR}/logs"}
: ${CACHE_DIR:="${SECRETS_DIR}/cache"}

# Setup session environment
export RENKU_MOUNT_DIR="${SESSION_WORK_DIR}"
export RENKU_WORKING_DIR="${SESSION_WORK_DIR}"
# Force the frontend to listen on 127.0.0.1
export RENKU_SESSION_IP="127.0.0.1"

# Do not leave secrets on a shared fs, move it to the node where the session runs.
mkdir -p "${SECRETS_DIR}"
chmod 700 "${SECRETS_DIR}"
mv "${SESSION_DIR}/secrets"/* "${SECRETS_DIR}"
rmdir "${SESSION_DIR}/secrets"

# Load the wstunnel secret
export WSTUNNEL_SECRET="$(cat "${SECRETS_DIR}/wstunnel_secret")"

# CamelCase to kebab-case conversion
#
# Usage:
#     kebab_cased="$(to_kebab_case "[cC]amelCaseInput")"
to_kebab_case() {
    echo "${1:?"to_kebab_case: input string missing"}" | sed 's/\([A-Z]\)/-\1/g' | tr '[:upper:]' '[:lower:]'
}

# Convert camelCased key - value pairs stored in a flat json struct to command line arguments.
#
# Usage:
#     arguments_string="$(to_rclone_mount_arguments "file_path" ["argument_prefix"])"
to_rclone_mount_arguments() {
    local filename=${1:?"to_arguments: input file missing"}
    local prefix=${2}
    cat "${filename}" | tr -d '{},"' | grep ':' | while IFS=': ' read -r key value; do
        printf "%s%s=%s " "${prefix}" "$(to_kebab_case "${key}")" "${value}"
    done
}

# Installs rclone
#
# Usage:
#     rclone="$(install_rclone "$version" "$gh_arch")"
#     "$rclone" version
function install_rclone() {
    rclone_version=${1:?"install_rclone: Version missing"}
    gh_arch=${2:?"install_rclone: Architecture missing"}
    rclone_pkg="${RENKU_PKG}/rclone/v${rclone_version}"
    rclone_bin="${rclone_pkg}/rclone"

    if [ -f "${rclone_bin}" ]; then
        version="$("${rclone_bin}" version || echo "bad executable")"
        version="$(echo "${version}" | head -n 1)"
        expected="rclone v${rclone_version}"
        if [ "${version}" = "${expected}" ]; then
            echo "${rclone_bin}"
            return 0
        else
            >&2 echo "WARNING: found mismatching rclone version ${version}"
        fi
    fi

    rclone_url="https://github.com/rclone/rclone/releases/download/v${rclone_version}/rclone-v${rclone_version}-linux-${gh_arch}.zip"

    mkdir -p "${rclone_pkg}"
    tmp="$(mktemp -d)"
    (# Run in a subshell to prevent changing the working directory of the caller
        cd "${tmp}"
        curl -Lo "rclone.zip" "${rclone_url}"
        >&2 unzip "rclone.zip"
        rm -rf "${rclone_pkg}"
        mv ./rclone-v"${rclone_version}"-* "${rclone_pkg}"
    )
    rm -r "${tmp}"
    chmod a+x "${rclone_bin}"

    echo "${rclone_bin}"
}

# Installs wstunnel
#
# Usage:
#     wstunnel="$(install_wstunnel "$version" "$gh_arch")"
#     "$wstunnel" --version
function install_wstunnel() {
    wstunnel_version=${1:?"wstunnel_version: Version missing"}
    gh_arch=${2:?"wstunnel_version: Architecture missing"}
    wstunnel_pkg="${RENKU_PKG}/wstunnel/v${wstunnel_version}"
    wstunnel_bin="${wstunnel_pkg}/wstunnel"

    >&2 echo "Info: using wstunnel v${wstunnel_version}"

    if [ -f "${wstunnel_bin}" ]; then
        version="$("${wstunnel_bin}" --version || echo "bad executable")"
        expected="wstunnel-cli ${wstunnel_version}"
        if [ "${version}" = "${expected}" ]; then
            echo "${wstunnel_bin}"
            return 0
        else
            >&2 echo "WARNING: found mismatching wstunnel version ${version}"
        fi
    fi

    wstunnel_url="https://github.com/SwissDataScienceCenter/wstunnel/releases/download/v${wstunnel_version}/wstunnel_${wstunnel_version}_linux_${gh_arch}.tar.gz"

    mkdir -p "${wstunnel_pkg}"
    tmp="$(mktemp -d)"
    (# Run in a sub shell to prevent changing the working directory of the caller
        cd "${tmp}"
        curl -Lo "wstunnel.tar.gz" "${wstunnel_url}"
        rm -rf "${wstunnel_pkg}"
        mkdir -p ${wstunnel_pkg} # the folder has to exist for tar -C
        tar xf "wstunnel.tar.gz" -C "${wstunnel_pkg}"
    )
    rm -r "${tmp}"
    chmod a+x "${wstunnel_bin}"

    echo "${wstunnel_bin}"
}

for d in \
    SESSION_DIR \
    SESSION_WORK_DIR \
    SECRETS_DIR \
    LOGS_DIR
do
    echo "${d}: ${!d}"
    mkdir -p "${!d}"
done

# Install rclone
rclone="$(install_rclone "${RCLONE_VERSION}" "${gh_arch}")"
echo "rclone: ${rclone}"

# Install wstunnel
wstunnel="$(install_wstunnel "${WSTUNNEL_VERSION}" "${gh_arch}")"
echo "wstunnel: ${wstunnel}"

# Ensure NVIDIA_VISIBLE_DEVICES is set to void
# so that cuda enabled images work on eiger
if !(nvidia-smi 2>&1 >/dev/null); then
    export NVIDIA_VISIBLE_DEVICES=void
fi

if srun --help | grep -q -- --environment; then
    # Create the environment.toml file to run the session
    EDF_FILE="${SESSION_DIR}/environment.toml"
    cat <<EOF >"${EDF_FILE}"
[annotations]
com.hooks.cxi.enabled = "false"
EOF
    srun_param_environment="--environment ${EDF_FILE}"
else
    srun_param_environment=""
fi

srun_param_container_image="--container-image ${REMOTE_SESSION_IMAGE}"
srun_param_workdir="--container-workdir ${SESSION_WORK_DIR}"
srun_param_mounts=#{{SESSION_MOUNTS_PLACEHOLDER}}
# We cannot generate directly the local secrets path from the proxy as the final path is only known after ${SECRETS_DIR} as been set.
srun_param_mounts=$(echo ${srun_param_mounts} | sed -e "s,${SESSION_DIR}/secrets,${SECRETS_DIR},g")

# Mount DataSources, if any
if [ -d  "${SECRETS_DATA_CONNECTORS_DIR}" ]; then
    (# Run in a sub shell to scope the temporary variables
        for dc in "${SECRETS_DATA_CONNECTORS_DIR}"/*; do
            n=$(echo ${dc}|sed -e 's,.*-,,')
            mount="$(cat "${dc}/remote")"
            remotePath="$(cat "${dc}/remotePath")"
            log_file="${LOGS_DIR}/rclone-dc-${n}.log"
            config_file="${dc}/configData"

            if [ -f "${dc}/vfsOpt" ]; then
                vfsOptions="$(to_rclone_mount_arguments "${dc}/vfsOpt" "--vfs")"
            fi

            if [ -f "${dc}/mountOpt" ]; then
                mountOptions="$(to_rclone_mount_arguments "${dc}/mountOpt" "-")"
            fi

            #readonly="--read-only" # force readonly for now

            echo >> "${log_file}"
            echo "--- Starting $(date)" >> "${log_file}"

            mkdir -p "${SESSION_WORK_DIR}/${mount}"
            mkdir -p "${CACHE_DIR}/${n}"

            ${rclone} mount \
                --daemon \
                ${mountOptions} \
                ${readonly} \
                --log-file="${log_file}" \
                --cache-dir="${CACHE_DIR}/$n" \
                ${vfsOptions} \
                --config="${config_file}" \
                "${mount}:${remotePath}" \
                "//${SESSION_WORK_DIR}/${mount}"
        done
    )
fi

# echo "Starting tunnel..."
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
    OIFS="${IFS}"
    IFS=$'\n'
    GIT_REPOSITORIES=(${GIT_REPOSITORIES})
    IFS="${OIFS}"

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
        echo "Git proxy not ready, cannot setup git repositories"
        for line in "${GIT_REPOSITORIES[@]}"; do
            repo="$(echo "${line}" | cut -d$'\t' -f1)"
            branch="$(echo "${line}" | cut -d$'\t' -f2)"
            echo "repo: ${repo}, branch: ${branch}"
            echo "Error: could not contact the git proxy" > "${RENKU_WORKING_DIR}/${repo}/ERROR"
        done
    else
        echo "Setting up git repositories..."
        cwd="$(pwd)"
        for line in "${GIT_REPOSITORIES[@]}"; do
            repo="$(echo "${line}" | cut -d$'\t' -f1)"
            branch="$(echo "${line}" | cut -d$'\t' -f2)"
            echo "repo: ${repo}, branch: ${branch}"
            cd "${RENKU_WORKING_DIR}/${repo}"
            git init || echo "Error: could not run git init" > "ERROR"
            git fetch || echo "Error: could not run git fetch" > "ERROR"
            if [ -n "${branch}" ]; then
                git checkout "${branch}"  || echo "Error: could not run git checkout" > "ERROR"
                git pull || echo "Error: could not run git pull" > "ERROR"
            fi
        done
        cd "${cwd}"
    fi
fi

function exit_script() {
    echo "Cleaning up session..."
    # Make sure we have a valid pid before attempting to kill it
    (test -n "${pid}" && ps "${pid}" > /dev/null && kill -TERM "${pid}") || true

    # kill rclone to unmount DCs, but only the ones of the current session
    # which we figure out by their log file
    ps -u "${USER}" -o pid=,cmd= | grep "${LOGS_DIR}/rclone-dc-" | grep -v grep | sed -e 's,^ *,,' | cut -d ' ' -f 1 | while read rclone_pid; do
        test -n "${rclone_pid}" | kill -TERM "${rclone_pid}" || true
    done

    # Cleanup Data Source mount points
    if [ -d "${SECRETS_DATA_CONNECTORS_DIR}" ]; then
        for dc in "${SECRETS_DATA_CONNECTORS_DIR}"/*; do
            rmdir "${SESSION_WORK_DIR}/$(cat "${dc}/remote")" || true
        done
    fi

    # Remove the secrets from the node, leaving them generates problem in case
    # of suppression of Data Connectors/user secrets from the project, for example.
    # Same thing for the caches, as they will be renumbered.
    for d in \
        CACHE_DIR \
        SECRETS_DATA_CONNECTORS_DIR \
        SECRETS_USER_DIR \
        SECRETS_DIR
    do
        rm -rf "${!d}" || true
    done

    # Sometimes the job continues to run...
    scancel "${SLURM_JOB_ID}" || true
}

echo "Starting session..."
# Start session while listening to EXIT signals
pid=
trap 'exit_script' EXIT
srun \
    ${srun_param_environment} \
    ${srun_param_container_image} \
    ${srun_param_workdir} \
    ${srun_param_mounts} \
    --no-container-entrypoint sh /etc/rc \
    & pid=$!
wait
pid=
