import pytest

from tests.integration.utils import find_resource


@pytest.mark.culling
def test_idle_culling(
    k8s_namespace,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
    k8s_pod_api,
    test_manifest,
    is_session_deleted,
):
    name = test_manifest["metadata"]["name"]
    launch_session(test_manifest)
    assert is_session_ready(name, timeout_mins=5)
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == name
    # wait for session to be culled
    is_session_deleted(
        name, test_manifest["spec"]["culling"]["idleSecondsThreshold"] + 60
    )
    # confirm session got culled
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
    assert (
        session is None
        or pod is None
        or pod["metadata"].get("deletionTimestamp") is not None
    )


@pytest.mark.culling
def test_failed_culling(
    k8s_namespace,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
    k8s_pod_api,
    test_manifest,
    is_session_deleted,
):
    name = test_manifest["metadata"]["name"]
    test_manifest["spec"]["resources"]["requests"]["memory"] = "999Ti"
    launch_session(test_manifest)
    assert not is_session_ready(name, timeout_mins=1)
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == name
    # wait for session to be culled
    is_session_deleted(
        name, test_manifest["spec"]["culling"]["failedSecondsThreshold"] + 30
    )
    # confirm session got culled
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
    assert (
        session is None
        or pod is None
        or pod["metadata"].get("deletionTimestamp") is not None
    )
