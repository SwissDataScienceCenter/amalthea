import base64
from datetime import datetime, timedelta
from os.path import expanduser
import tempfile
from time import sleep
from uuid import uuid4
import os
from subprocess import Popen

import pytest
from kubernetes import config, client
from kubernetes.dynamic import DynamicClient
from kubernetes.dynamic.exceptions import NotFoundError
import yaml

from controller.culling import get_js_server_status
from tests.integration.utils import find_resource
from utils.chart_rbac import cleanup_k8s_resources, create_k8s_resources


@pytest.fixture(scope="session")
def operator_env(operator_kubeconfig_fp):
    """Use to override environment variables that set the kube config.
    But also (and more importantly) this can be used to override important
    application-level operator configuration like the interval at which
    the idle checks run. Refer to the config.py in the controler folder for
    available environment variables that can be overriden."""
    yield {
        "KUBECONFIG": operator_kubeconfig_fp.name,
        "JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS": "5",
        "JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS": "5",
    }


@pytest.fixture(scope="session", autouse=True)
def operator(k8s_namespace, operator_env, k8s_amalthea_api, kopf_log_files_fp):
    stdout, stderr = kopf_log_files_fp
    p = Popen(
        args=f"kopf run -n {k8s_namespace} --verbose kopf_entrypoint.py",
        stdout=stdout,
        stderr=stderr,
        shell=True,
        env={
            **os.environ,
            **operator_env,
        },
        bufsize=0,
        # NOTE: os.setpgrp makes it so that Ctrl-C is ignored.
        # This is needed because Ctrl-C gets picked up if the tests are stopped manually
        # But stopping the operator the instant the tests are stopped results in
        # incomplete cleanup of fixtures and leftover k8s resources
        preexec_fn=os.setpgrp,
    )
    # INFO: Give time for the operator to fully start up, call to Popen does not block
    sleep(10)
    yield p

    # NOTE: Keep the kopf runner (i.e. operator) going until all sessions are cleaned up
    sessions = k8s_amalthea_api.get(namespace=k8s_namespace)
    print("Checking for the number of active sessions")
    while len(sessions.items) > 0:
        sessions = k8s_amalthea_api.get(namespace=k8s_namespace)
        print(f"{len(sessions.items)} active sessions found")
        sleep(10)
    # NOTE: Ctrl-C is ignored - this is only way to stop operator
    p.kill()
    # INFO: Extract and post logs from controller - in most cases controller writes
    # to standard error regardless of whether erors were present or not
    stdout.seek(0)
    stderr.seek(0)
    stdout_content = stdout.read()
    stderr_content = stderr.read()
    if type(stdout_content) is bytes:
        stdout = stdout_content.decode()
    if type(stderr_content) is bytes:
        stderr_content = stderr_content.decode()
    try:
        term_width = os.get_terminal_size().columns
    except OSError:
        term_width = 80
    print(" KOPF STDOUT ".center(term_width, "*"))
    print(stdout_content)
    print(" KOPF STDERR ".center(term_width, "*"))
    print(stderr_content)
    print("*".center(term_width, "*"))


@pytest.fixture(scope="session", autouse=True)
def operator_kubeconfig_fp():
    with tempfile.NamedTemporaryFile("w") as fout:
        yield fout


@pytest.fixture(scope="session", autouse=True)
def kopf_log_files_fp():
    with tempfile.NamedTemporaryFile("w+b") as stdout, tempfile.NamedTemporaryFile(
        "w+b"
    ) as stderr:
        yield stdout, stderr


@pytest.fixture(scope="session", autouse=True)
def make_operator_kubeconfig(
    create_amalthea_k8s_resources,
    k8s_namespace,
    operator_kubeconfig_fp,
    release_name,
):
    k8s_client = client.ApiClient()
    # Get the token for the amalthea service account
    dc = DynamicClient(k8s_client)
    sa_api = dc.resources.get(api_version="v1", kind="ServiceAccount")
    sa = sa_api.get(f"{release_name}", k8s_namespace)
    secret_api = dc.resources.get(api_version="v1", kind="Secret")
    sa_token_secret = secret_api.get(sa["secrets"][0]["name"], k8s_namespace)
    token = base64.b64decode(sa_token_secret["data"]["token"].encode()).decode()

    # Read kube config, replace current context, save to file
    kc_path = config.KUBE_CONFIG_DEFAULT_LOCATION
    if kc_path.startswith("~"):
        kc_path = expanduser("~") + kc_path[1:]
    with open(kc_path, "r") as f:
        kc = yaml.safe_load(f)
    current_context = kc["current-context"]
    for iuser, user in enumerate(kc["users"]):
        if user["name"] == current_context:
            kc["users"][iuser]["user"] = {"token": token}
            kc["users"] = [user]
    for _, cluster in enumerate(kc["clusters"]):
        if cluster["name"] == current_context:
            kc["clusters"] = [cluster]
    for _, context in enumerate(kc["contexts"]):
        if context["name"] == current_context:
            kc["contexts"] = [context]
    yaml.dump(kc, operator_kubeconfig_fp)


@pytest.fixture
def read_manifest():
    def _read_manifest(manifest):
        with open(manifest, "r") as f:
            spec = yaml.safe_load(f)
        return spec

    return _read_manifest


@pytest.fixture(scope="session", autouse=True)
def load_k8s_config():
    config.load_kube_config()


@pytest.fixture(scope="session")
def get_k8s_api(load_k8s_config):
    apis = {}

    def _get_k8s_api(api_version, kind):
        if (api_version, kind) in apis.keys():
            return apis[(api_version, kind)]
        else:
            k8s_client = DynamicClient(client.ApiClient())
            api = k8s_client.resources.get(api_version=api_version, kind=kind)
            apis[(api_version, kind)] = api
            return api

    yield _get_k8s_api


@pytest.fixture(scope="session")
def k8s_amalthea_api(get_k8s_api, create_amalthea_k8s_resources):
    yield get_k8s_api(
        "v1alpha1",
        "JupyterServer",
    )


@pytest.fixture
def k8s_pod_api(get_k8s_api):
    yield get_k8s_api("v1", "Pod")


@pytest.fixture(scope="session")
def k8s_namespace():
    yield "default"


@pytest.fixture(scope="session")
def release_name():
    yield "amalthea-test"


@pytest.fixture
def launch_session(k8s_amalthea_api, k8s_namespace, is_session_deleted):
    launched_sessions = []

    def _launch_session(manifest):
        k8s_amalthea_api.create(manifest, namespace=k8s_namespace)
        launched_sessions.append(manifest)

    yield _launch_session

    # cleanup
    for session in launched_sessions:
        print(f"\nCleaning up session {session['metadata']['name']}")
        try:
            k8s_amalthea_api.delete(
                session["metadata"]["name"],
                namespace=k8s_namespace,
                propagation_policy="Foreground",
                async_req=False,
            )
            is_session_deleted(session["metadata"]["name"])
        except NotFoundError:
            pass
        else:
            print("Finished cleaning up sesssion.")


@pytest.fixture
def is_session_ready(k8s_namespace, k8s_amalthea_api):
    def _is_session_ready(name, timeout_mins=5):
        """The session is considered ready only when it successfully responds
        to a status request."""
        tstart = datetime.now()
        timeout = timedelta(minutes=timeout_mins)
        while datetime.now() - tstart < timeout:
            session = find_resource(name, k8s_namespace, k8s_amalthea_api)
            if session is not None:
                try:
                    status = get_js_server_status(session)
                except KeyError:
                    return False
                if status is not None:
                    return True
            sleep(2)
        return False

    yield _is_session_ready


@pytest.fixture
def is_session_deleted(k8s_namespace, k8s_pod_api, k8s_amalthea_api):
    def _is_session_deleted(name, timeout_mins=5):
        """Has the session been fully shut down"""
        tstart = datetime.now()
        timeout = timedelta(minutes=timeout_mins)
        while datetime.now() - tstart < timeout:
            pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
            session = find_resource(name, k8s_namespace, k8s_amalthea_api)
            if pod is not None or session is not None:
                sleep(2)
            else:
                return True
        return False

    yield _is_session_deleted


@pytest.fixture(scope="session", autouse=True)
def create_amalthea_k8s_resources(load_k8s_config, release_name, k8s_namespace):
    """This fixture configures the tests to use a serviceaccount
    with the same roles that the operator has when installed through
    the helm chart."""
    print("Creating custom resources for Amalthea.")
    yield create_k8s_resources(
        k8s_namespace,
        [k8s_namespace],
        resources=["ServiceAccount", "Role", "RoleBinding", "CustomResourceDefinition"],
        release_name=release_name,
    )
    print("Removing custom resources for Amalthea.")
    # Cleanup after testing
    cleanup_k8s_resources(
        k8s_namespace,
        [k8s_namespace],
        resources=["ServiceAccount", "Role", "RoleBinding", "CustomResourceDefinition"],
        release_name=release_name,
    )


@pytest.fixture
def custom_session_manifest(read_manifest, k8s_namespace):
    def _custom_session_manifest(
        manifest_file="tests/examples/token.yaml",
        name=f"test-session-{uuid4()}",
        jupyter_server={"image": "jupyter/minimal-notebook:latest"},
        routing={},
        culling={
            "idleSecondsThreshold": 0,
            "startingSecondsThreshold": 0,
            "failedSecondsThreshold": 0,
            "maxAgeSecondsThreshold": 0,
        },
        auth={
            "token": "test-auth-token",
            "oidc": {
                "enabled": False,
            },
        },
    ):
        manifest = read_manifest(manifest_file)
        return {
            **manifest,
            "metadata": {
                "name": name,
                "namespace": k8s_namespace,
            },
            "spec": {
                "auth": auth,
                "culling": culling,
                "jupyterServer": jupyter_server,
                "routing": {"host": "localhost", **routing},
            },
        }

    yield _custom_session_manifest


@pytest.fixture(
    params=[
        {
            "auth": {
                "token": "",
                "oidc": {
                    "enabled": True,
                    "issuerUrl": "https://accounts.google.com",
                    "clientId": "amalthea-test-session",
                    "clientSecret": {
                        "value": "amalthea-test-session-secret",
                    },
                },
            },
            "manifest_file": "tests/examples/oidc.yaml",
        },
        {
            "auth": {
                "token": "test-token-123",
                "oidc": {
                    "enabled": False,
                },
            },
            "manifest_file": "tests/examples/token.yaml",
        },
    ],
    ids=["oidc_auth", "token_auth"],
)
def test_manifest(request, custom_session_manifest):
    yield custom_session_manifest(
        manifest_file=request.param["manifest_file"],
        auth=request.param["auth"],
    )


@pytest.fixture
def patch_sleep_init_container():
    def _patch_sleep_init_container(sleep_duration_seconds):
        return {
            "type": "application/json-patch+json",
            "patch": [
                {
                    "op": "add",
                    "path": "/statefulset/spec/template/spec/initContainers/-",
                    "value": {
                        "image": "busybox",
                        "name": "sleep",
                        "command": ["sleep"],
                        "args": [str(sleep_duration_seconds)],
                    },
                },
            ],
        }
    yield _patch_sleep_init_container
