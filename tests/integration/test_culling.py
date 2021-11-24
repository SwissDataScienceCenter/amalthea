import pytest
from time import sleep

from tests.integration.utils import find_resource


@pytest.mark.culling
def test_(
    k8s_namespace,
    operator,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
    test_manifest,
):
    name = test_manifest["metadata"]["name"]
    operator = launch_session(test_manifest)
    # confirm session successfully launched
    assert operator.exit_code == 0
    assert operator.exception is None
    pod = is_session_ready(name)
    assert pod is not None
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == name
    # wait for session to be culled
    sleep(test_manifest["spec"]["culling"]["idleSecondsThreshold"] + 60)
    # confirm session got culled
    assert find_resource(name, k8s_namespace, k8s_amalthea_api) is None
