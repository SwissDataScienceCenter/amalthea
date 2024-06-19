import random
import string
import threading
from collections.abc import Iterator
from datetime import datetime, timedelta
from time import sleep
from typing import Any
from uuid import uuid4

import pytest
import uvloop
import yaml
from kubernetes import config
from kubernetes.client import (
    ApiClient,
    CoreV1Api,
    V1DeleteOptions,
    V1Namespace,
    V1ObjectMeta,
)
from kubernetes.dynamic import DynamicClient
from kubernetes.dynamic.exceptions import NotFoundError, ResourceNotFoundError
from kubernetes.dynamic.resource import Resource

from controller.crds import jupyter_server_crd_dict
from controller.culling import get_js_server_status
from controller.main import run as run_controller
from tests.integration.utils import find_resource


@pytest.fixture
def k8s_crd_api(k8s_dynamic: DynamicClient) -> Resource:
    crd_api = k8s_dynamic.resources.get(api_version="apiextensions.k8s.io/v1", kind="CustomResourceDefinition")
    return crd_api


@pytest.fixture
def read_manifest():
    def _read_manifest(manifest):
        with open(manifest, "r") as f:
            spec = yaml.safe_load(f)
        return spec

    return _read_manifest


@pytest.fixture
def k8s_core() -> CoreV1Api:
    config.load_kube_config()
    k8s_client = CoreV1Api()
    return k8s_client


@pytest.fixture
def k8s_dynamic() -> DynamicClient:
    config.load_kube_config()
    api_client = ApiClient()
    k8s_client = DynamicClient(api_client)
    return k8s_client


@pytest.fixture
def k8s_pod_api(k8s_dynamic: DynamicClient) -> Resource:
    return k8s_dynamic.resources.get(api_version="v1", kind="Pod")


@pytest.fixture
def js_crd_manifest() -> Iterator[dict[str, Any]]:
    manifest = jupyter_server_crd_dict()
    random_prefix = "test" + "".join(random.choices(string.ascii_lowercase + string.digits, k=6))
    manifest["metadata"]["name"] = random_prefix + manifest["metadata"]["name"]
    manifest["spec"]["names"]["kind"] = random_prefix + manifest["spec"]["names"]["kind"]
    manifest["spec"]["names"]["plural"] = random_prefix + manifest["spec"]["names"]["plural"]
    manifest["spec"]["names"]["singular"] = random_prefix + manifest["spec"]["names"]["singular"]
    manifest["spec"]["names"]["shortNames"] = []
    return manifest


@pytest.fixture
def k8s_namespace(k8s_core: CoreV1Api) -> Iterator[str]:
    ns = "ns-" + str(uuid4())
    k8s_core.create_namespace(V1Namespace(metadata=V1ObjectMeta(name=ns)))
    yield ns
    k8s_core.delete_namespace(name=ns, propagation_policy="Foreground")


@pytest.fixture
def operator(
    k8s_namespace: str,
    monkeypatch,
    k8s_crd_api: Resource,
    mocker,
    js_crd_manifest: dict[str, Any],
    k8s_dynamic: DynamicClient,
) -> Iterator[Resource]:
    # Create CRD in K8s
    k8s_crd_api.create(js_crd_manifest)
    while True:
        try:
            k8s_crd_api.get(name=js_crd_manifest["metadata"]["name"])
        except NotFoundError:
            sleep(1)
        else:
            break
    k8s_dynamic.resources.invalidate_cache()
    k8s_dynamic.resources.discover()
    # Get a client
    retries = 0
    while True:
        retries += 1
        if retries > 5:
            assert False, "cannot initialize the operator"
        try:
            js_api = k8s_dynamic.resources.get(
                group=js_crd_manifest["spec"]["group"],
                api_version=js_crd_manifest["spec"]["versions"][0]["name"],
                kind=js_crd_manifest["spec"]["names"]["kind"],
            )
        except ResourceNotFoundError:
            sleep(1)
        else:
            break
    # Setup oprerator environment / config
    monkeypatch.setenv("CRD_API_GROUP", js_api.group)
    monkeypatch.setenv("CRD_API_VERSION", js_api.api_version)
    monkeypatch.setenv("CRD_NAME", js_api.kind)
    monkeypatch.setenv("NAMESPACES", k8s_namespace)
    mocker.patch("controller.config.api_group", js_api.group)
    mocker.patch("controller.config.api_version", js_api.api_version)
    mocker.patch("controller.config.custom_resource_name", js_api.kind)
    mocker.patch("controller.config.NAMESPACES", [k8s_namespace])
    mocker.patch("controller.config.JUPYTER_SERVER_PENDING_CHECK_INTERVAL_SECONDS", 2)
    mocker.patch("controller.config.JUPYTER_SERVER_IDLE_CHECK_INTERVAL_SECONDS", 2)
    stop_flag = threading.Event()
    ready_flag = threading.Event()
    thread = threading.Thread(
        target=uvloop.run,
        args=(run_controller(stop_flag=stop_flag, ready_flag=ready_flag),),
    )
    thread.start()
    ready_flag.wait()
    yield js_api
    # Cleanup
    resources = js_api.get()
    for res in resources.items:
        js_name = res["metadata"]["name"]
        js_namespace = res["metadata"]["namespace"]
        try:
            js_api.delete(
                name=js_name,
                namespace=js_namespace,
                body=V1DeleteOptions(propagation_policy="Foreground"),
            )
        except NotFoundError:
            pass
    # Wait for all jupyter servers to be deleted before quitting the operator
    # If not jupyter servers get orphaned and the CRD and namespaces cannot be deleted
    # For some reason the Foreground propagation policies that are supposed to prevent this
    # do not seem to help at all and jupyter servers still get orphaned without this loop.
    while js_api.get().items:
        sleep(0.5)
    stop_flag.set()
    thread.join(timeout=10)
    k8s_crd_api.delete(
        name=js_crd_manifest["metadata"]["name"],
        body=V1DeleteOptions(propagation_policy="Foreground"),
    )


@pytest.fixture
def is_session_ready(k8s_namespace: str, operator: Resource):
    def _is_session_ready(name, timeout_mins=5):
        """The session is considered ready only when it successfully responds
        to a status request."""
        tstart = datetime.now()
        timeout = timedelta(minutes=timeout_mins)
        while datetime.now() - tstart < timeout:
            session = find_resource(name, k8s_namespace, operator)
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
def is_session_deleted(k8s_namespace, k8s_pod_api: Resource, operator: Resource):
    def _is_session_deleted(name, timeout=300):
        """Has the session been fully shut down"""
        tstart = datetime.now()
        timeout = timedelta(seconds=timeout)
        while datetime.now() - tstart < timeout:
            pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
            session = find_resource(name, k8s_namespace, operator)
            if pod is not None or session is not None:
                sleep(2)
            else:
                return True
        return False

    yield _is_session_deleted


@pytest.fixture
def wait_for_pod_deletion(k8s_namespace, k8s_pod_api):
    def is_pod_deleted(name, timeout):
        end = datetime.now() + timedelta(seconds=timeout)
        while datetime.now() < end:
            pod = find_resource(f"{name}-0", k8s_namespace, k8s_pod_api)
            if not pod:
                return True
            sleep(2)
        return False

    yield is_pod_deleted


@pytest.fixture
def custom_session_manifest(read_manifest, k8s_namespace: str, operator: Resource):
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
            "hibernatedSecondsThreshold": 0,
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
            "apiVersion": operator.group_version,
            "kind": operator.kind,
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
