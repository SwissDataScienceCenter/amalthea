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
    culling_threshold_seconds = 60
    test_manifest["spec"]["culling"]["idleSecondsThreshold"] = culling_threshold_seconds
    launch_session(test_manifest)
    assert is_session_ready(name, timeout_mins=5)
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session["spec"]["culling"]["idleSecondsThreshold"] == culling_threshold_seconds
    assert session["spec"]["culling"]["startingSecondsThreshold"] == 0
    assert session["spec"]["culling"]["failedSecondsThreshold"] == 0
    assert session["spec"]["culling"]["maxAgeSecondsThreshold"] == 0
    assert session is not None
    assert session["metadata"]["name"] == name
    # wait for session to be culled
    is_session_deleted(name, culling_threshold_seconds + 60)
    # confirm session got culled
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
    assert session is None or pod is None or pod["metadata"].get("deletionTimestamp") is not None


@pytest.mark.culling
def test_starting_culling(
    k8s_namespace,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
    k8s_pod_api,
    test_manifest,
    is_session_deleted,
    patch_sleep_init_container,
):
    name = test_manifest["metadata"]["name"]
    culling_threshold_seconds = 60
    test_manifest["spec"]["culling"]["startingSecondsThreshold"] = culling_threshold_seconds
    test_manifest["spec"]["patches"] = [patch_sleep_init_container(300)]
    launch_session(test_manifest)
    assert not is_session_ready(name, timeout_mins=1)
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session["spec"]["culling"]["idleSecondsThreshold"] == 0
    assert session["spec"]["culling"]["startingSecondsThreshold"] == culling_threshold_seconds
    assert session["spec"]["culling"]["failedSecondsThreshold"] == 0
    assert session["spec"]["culling"]["maxAgeSecondsThreshold"] == 0
    assert session is not None
    assert session["metadata"]["name"] == name
    # wait for session to be culled
    is_session_deleted(name, culling_threshold_seconds + 60)
    # confirm session got culled
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
    assert session is None or pod is None or pod["metadata"].get("deletionTimestamp") is not None


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
    culling_threshold_seconds = 60
    test_manifest["spec"]["jupyterServer"]["image"] = "nginx:latest"
    test_manifest["spec"]["culling"]["failedSecondsThreshold"] = culling_threshold_seconds
    launch_session(test_manifest)
    assert not is_session_ready(name, timeout_mins=1)
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session["spec"]["culling"]["idleSecondsThreshold"] == 0
    assert session["spec"]["culling"]["startingSecondsThreshold"] == 0
    assert session["spec"]["culling"]["failedSecondsThreshold"] == culling_threshold_seconds
    assert session["spec"]["culling"]["maxAgeSecondsThreshold"] == 0
    assert session is not None
    assert session["metadata"]["name"] == name
    # wait for session to be culled
    is_session_deleted(name, culling_threshold_seconds + 60)
    # confirm session got culled
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    pod = find_resource(name + "-0", k8s_namespace, k8s_pod_api)
    assert session is None or pod is None or pod["metadata"].get("deletionTimestamp") is not None
