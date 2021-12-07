import base64
from datetime import datetime, timedelta
from os.path import expanduser
import tempfile
from time import sleep
from uuid import uuid4
import os
from subprocess import Popen, TimeoutExpired
import sys

import pytest
from kubernetes import config, client
from kubernetes.dynamic import DynamicClient
from kubernetes.dynamic.exceptions import NotFoundError
import yaml

from controller.culling import get_js_server_status
from tests.integration.utils import find_resource
from utils.chart_rbac import cleanup_local_dev, create_k8s_resources, RELEASE_NAME


@pytest.fixture(scope="session")
def operator_env(operator_kubeconfig_fp):
    """Use to override environment variables that set the kube config.
    But also (and more importantly) this can be used to override important
    application-level operator configuration like the interval at which
    the idle checks run. Refer to the config.py in the controler folder for
    available environment variables that can be overriden."""
    yield {
        "KUBECONFIG": operator_kubeconfig_fp.name,
        "JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS": "5",
    }


@pytest.fixture(scope="session", autouse=True)
def operator(k8s_namespace, operator_env):
    p = Popen(
        args=f"kopf run -n {k8s_namespace} --verbose kopf_entrypoint.py",
        stdout=sys.stdout,
        stderr=sys.stderr,
        shell=True,
        env={
            **os.environ,
            **operator_env,
        },
        bufsize=0,
    )
    # Give time for the operator to fully start up, call to Popen does not block
    sleep(2)
    yield p

    # We run the KopfRunner again for a short moment to remove all finalizers
    # and allow cleanup.
    sleep(10)
    p.terminate()
    try:
        p.wait(timeout=30)
    except TimeoutExpired:
        p.kill()


@pytest.fixture(scope="session", autouse=True)
def operator_kubeconfig_fp():
    return tempfile.NamedTemporaryFile("w")


@pytest.fixture(scope="session", autouse=True)
def make_operator_kubeconfig(
    create_amalthea_k8s_resources,
    k8s_namespace,
    operator_kubeconfig_fp,
):
    k8s_client = client.ApiClient()
    # Get the token for the amalthea service account
    dc = DynamicClient(k8s_client)
    sa_api = dc.resources.get(api_version="v1", kind="ServiceAccount")
    sa = sa_api.get(f"{RELEASE_NAME}-amalthea", k8s_namespace)
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


@pytest.fixture
def k8s_amalthea_api(get_k8s_api):
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


@pytest.fixture
def launch_session(k8s_amalthea_api, k8s_namespace):
    launched_sessions = []

    def _launch_session(manifest):
        k8s_amalthea_api.create(manifest, namespace=k8s_namespace)
        launched_sessions.append(manifest)

    yield _launch_session

    # cleanup
    for session in launched_sessions:
        try:
            k8s_amalthea_api.delete(
                session["metadata"]["name"],
                namespace=k8s_namespace,
                propagation_policy="Foreground",
                async_req=False,
            )
        except NotFoundError:
            pass


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
def is_session_deleted(k8s_namespace, k8s_pod_api):
    def _is_session_deleted(name, timeout_mins=5):
        """Has the session been fully shut down"""
        tstart = datetime.now()
        timeout = timedelta(minutes=timeout_mins)
        while datetime.now() - tstart < timeout:
            pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
            if pod is not None:
                sleep(2)
            else:
                return True
        return False

    yield _is_session_deleted


@pytest.fixture(scope="session", autouse=True)
def create_amalthea_k8s_resources(load_k8s_config):
    """This fixture configures the tests to use a serviceaccount
    with the same roles that the operator has when installed through
    the helm chart."""

    yield create_k8s_resources(
        "default",
        ["default"],
        resources=["ServiceAccount", "Role", "RoleBinding", "CustomResourceDefinition"],
    )

    # Cleanup after testing
    cleanup_local_dev(
        "default",
        ["default"],
        resources=["ServiceAccount", "Role", "RoleBinding", "CustomResourceDefinition"],
    )


@pytest.fixture
def custom_session_manifest(read_manifest, k8s_namespace):
    def _custom_session_manifest(
        manifest_file="tests/examples/token.yaml",
        name=f"test-session-{uuid4()}",
        jupyter_server={"image": "jupyter/minimal-notebook:latest"},
        routing={},
        culling={"idleSecondsThreshold": 30},
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
