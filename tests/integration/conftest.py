from datetime import datetime, timedelta
from pathlib import Path
import subprocess
import tempfile
from time import sleep

import pytest
from kopf.testing import KopfRunner
from kubernetes import config
from kubernetes.dynamic import DynamicClient
import kubernetes.client as k8s_client
import yaml

from tests.integration.utils import find_resource, is_pod_ready
from utils.chart_rbac import configure_local_dev, cleanup_local_dev


@pytest.fixture
def operator(configure_rbac, k8s_namespace):
    yield KopfRunner(
        [
            "run",
            "-n",
            f"{k8s_namespace}",
            "--verbose",
            "kopf_entrypoint.py",
        ]
    )

    # We run the KopfRunner again for a short moment to remove all finalizers
    # and allow cleanup.
    with KopfRunner(
        [
            "run",
            "-n",
            f"{k8s_namespace}",
            "kopf_entrypoint.py",
        ]
    ):
        sleep(2)


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
        with operator as runner:
            k8s_amalthea_api.create(manifest, namespace=k8s_namespace)
            is_session_ready(manifest["metadata"]["name"])
            launched_sessions.append(manifest)
        return runner

    yield _launch_session

    # cleanup
    for session in launched_sessions:
        k8s_amalthea_api.delete(
            session["metadata"]["name"],
            namespace=k8s_namespace,
            propagation_policy="Foreground",
            async_req=False,
        )


@pytest.fixture
def is_session_ready(k8s_amalthea_api, k8s_namespace, k8s_pod_api):
    def _is_session_ready(name, timeout_mins=10):
        tstart = datetime.now()
        timeout = timedelta(minutes=timeout_mins)
        session = None
        pod_fully_ready = False
        pod = None
        while (
            datetime.now() - tstart < timeout
            and session is None
            and not pod_fully_ready
        ):
            session = find_resource(name, k8s_namespace, k8s_amalthea_api)
            pod_name = name + "-0"
            pod = find_resource(pod_name, k8s_namespace, k8s_pod_api)
            if pod is not None:
                pod_fully_ready = is_pod_ready(pod)
                if pod_fully_ready:
                    return pod
            sleep(5)
        return pod

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
