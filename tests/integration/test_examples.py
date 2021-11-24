import pytest

from tests.integration.utils import find_resource


@pytest.mark.examples
def test_example(
    k8s_namespace,
    operator,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
    test_manifest,
):
    name = test_manifest["metadata"]["name"]
    operator = launch_session(test_manifest)
    assert operator.exit_code == 0
    assert operator.exception is None
    pod = is_session_ready(name, timeout_mins=2)
    assert pod is not None
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == test_manifest["metadata"]["name"]
    assert session["spec"]["routing"]["host"] == test_manifest["spec"]["routing"]["host"]
    if test_manifest["spec"]["auth"]["oidc"]["enabled"]:
        assert session["spec"]["auth"]["token"] == test_manifest["spec"]["auth"]["token"]
