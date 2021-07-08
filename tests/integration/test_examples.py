from tests.integration.utils import find_resource


def test_oidc_example(
    read_manifest,
    k8s_namespace,
    operator,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
):
    manifest = read_manifest("examples/oidc.yaml")
    name = "test-oidc"
    host = "test.host.com"
    manifest["metadata"]["name"] = name
    manifest["metadata"]["namespace"] = k8s_namespace
    manifest["spec"]["routing"] = {"host": host}
    operator = launch_session(manifest)
    assert operator.exit_code == 0
    assert operator.exception is None
    pod = is_session_ready(name)
    assert pod is not None
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == name
    assert session["spec"]["routing"]["host"] == host


def test_token_example(
    read_manifest,
    k8s_namespace,
    operator,
    launch_session,
    is_session_ready,
    k8s_amalthea_api,
):
    manifest = read_manifest("examples/token.yaml")
    name = "test-token"
    host = "test.host.com"
    token = "secret_token_123456"
    manifest["metadata"]["name"] = name
    manifest["metadata"]["namespace"] = k8s_namespace
    manifest["spec"]["routing"] = {"host": host}
    manifest["spec"]["auth"]["token"] = token
    operator = launch_session(manifest)
    assert operator.exit_code == 0
    assert operator.exception is None
    pod = is_session_ready(name)
    assert pod is not None
    session = find_resource(name, k8s_namespace, k8s_amalthea_api)
    assert session is not None
    assert session["metadata"]["name"] == name
    assert session["spec"]["routing"]["host"] == host
    assert session["spec"]["auth"]["token"] == token
