import pytest

from tests.integration.utils import find_resource


@pytest.mark.examples
def test_example(
    k8s_namespace,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
    test_manifest,
):
    name = test_manifest["metadata"]["name"]
    test_manifest["spec"]["culling"] = {
        "idleSecondsThreshold": 0,  # disable culling
    }
    launch_session(test_manifest)
    assert is_session_ready(name, timeout_mins=5)
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == test_manifest["metadata"]["name"]
    assert session["spec"]["routing"]["host"] == test_manifest["spec"]["routing"]["host"]
    if test_manifest["spec"]["auth"]["oidc"]["enabled"]:
        assert session["spec"]["auth"]["token"] == test_manifest["spec"]["auth"]["token"]
