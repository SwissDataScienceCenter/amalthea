from kubernetes.dynamic.resource import Resource
import pytest

from tests.integration.utils import find_resource


@pytest.mark.examples
def test_example(
    k8s_namespace: str,
    is_session_ready,
    operator: Resource,
    test_manifest,
):
    name = test_manifest["metadata"]["name"]
    test_manifest["spec"]["culling"] = {
        "idleSecondsThreshold": 0,  # disable culling
    }
    operator.create(test_manifest, namespace=k8s_namespace)
    assert is_session_ready(name, timeout_mins=5)
    session = find_resource(name, k8s_namespace, operator)
    assert session is not None
    assert session["metadata"]["name"] == test_manifest["metadata"]["name"]
    assert session["spec"]["routing"]["host"] == test_manifest["spec"]["routing"]["host"]
    if test_manifest["spec"]["auth"]["oidc"]["enabled"]:
        assert session["spec"]["auth"]["token"] == test_manifest["spec"]["auth"]["token"]
