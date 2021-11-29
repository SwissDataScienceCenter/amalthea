from datetime import datetime, timedelta
from pathlib import Path
import subprocess
import tempfile
from time import sleep
from uuid import uuid4

import pytest
from kopf.testing import KopfRunner
from kubernetes import config
from kubernetes.dynamic import DynamicClient
from kubernetes.dynamic.exceptions import NotFoundError
import kubernetes.client as k8s_client
import yaml

from controller.culling import get_js_server_status
from tests.integration.utils import find_resource
from utils.chart_rbac import configure_local_dev, cleanup_local_dev


@pytest.fixture(scope="session", autouse=True)
def operator(k8s_namespace, configure_rbac, install_crd):
    # start the operator
    kopf_runner = KopfRunner(
        [
            "run",
            "-n",
            f"{k8s_namespace}",
            "--verbose",
            "kopf_entrypoint.py",
        ]
    ).__enter__()

    def _operator():
        return kopf_runner

    yield _operator

    # shut down the operator, sleep a bit to allow for proper cleanup
    sleep(10)
    kopf_runner.__exit__()


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
            client = DynamicClient(k8s_client.ApiClient())
            api = client.resources.get(api_version=api_version, kind=kind)
            apis[(api_version, kind)] = api
            return api

    yield _get_k8s_api


@pytest.fixture
def k8s_amalthea_api(get_k8s_api, install_crd):
    yield get_k8s_api(
        "v1alpha1",
        "JupyterServer",
    )


@pytest.fixture
def k8s_pod_api(get_k8s_api):
    yield get_k8s_api("v1", "Pod")


@pytest.fixture
def k8s_namespace():
    yield "default"


@pytest.fixture
def launch_session(operator, k8s_amalthea_api, k8s_namespace, is_session_ready):
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
def is_session_ready(k8s_namespace, k8s_amalthea_api, k8s_pod_api):
    def _is_session_ready(name, timeout_mins=5):
        """The session is considered ready only when it successfully responds
        to a status request."""
        tstart = datetime.now()
        timeout = timedelta(minutes=timeout_mins)
        while (datetime.now() - tstart < timeout):
            session = find_resource(name, k8s_namespace, k8s_amalthea_api)
            if session is not None:
                try:
                    status = get_js_server_status(session)
                except KeyError:
                    return False
                if status is not None:
                    return True
            sleep(10)
        return False

    yield _is_session_ready


@pytest.fixture(scope="session", autouse=True)
def install_crd(load_k8s_config):
    crd_file = f"{tempfile.mkdtemp()}/crd.yaml"
    manifest_str = subprocess.check_output(
        ["helm", "template", "amalthea", "helm-chart/amalthea"]
    )
    manifest = yaml.safe_load_all(manifest_str)
    with open(crd_file, "w") as f:
        crd = [spec for spec in manifest if spec["kind"] == "CustomResourceDefinition"]
        yaml.dump(crd[0], f)
    yield subprocess.check_call(["kubectl", "apply", "-f", crd_file])

    subprocess.check_call(["kubectl", "delete", "-f", crd_file])
    Path(crd_file).unlink(missing_ok=True)


@pytest.fixture(scope="session", autouse=True)
def configure_rbac(install_crd):
    """This fixture configures the tests to use a serviceaccount
    with the same roles that the operator has when installed through
    the helm chart."""

    admin_context = subprocess.check_output(
        "kubectl config current-context", shell=True
    ).decode()
    yield configure_local_dev("default", ["default"], include_crd=False)

    # Cleanup after testing
    cleanup_local_dev(admin_context, "default", ["default"], include_crd=False)


@pytest.fixture
def custom_session_manifest(read_manifest, k8s_namespace):
    def _custom_session_manifest(
        manifest_file="tests/examples/token.yaml",
        name=f"test-session-{uuid4()}",
        jupyter_server={"image": "jupyter/minimal-notebook:latest"},
        routing={},
        culling={"idleSecondsThreshold": 180},
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
                "routing": {
                    "host": f"{name}.{k8s_namespace}",
                    **routing
                },
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
                    }
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
