import pytest
import re

from controller.k8s_resources import (
    get_urls,
    get_children_templates,
    create_template_values,
)


@pytest.mark.parametrize("tls", [{"enabled": True}, {"enabled": False}])
def test_get_urls(tls, valid_spec):
    routing = {
        "host": "test.host",
        "path": "test_path",
        "tls": tls,
        "ingressAnnotations": {},
    }
    spec = valid_spec(routing=routing)
    host_url, full_url = get_urls(spec)
    if tls["enabled"]:
        assert host_url.startswith("https")
    else:
        assert host_url.startswith("http")
    re_match = re.match(
        r"^http[s]*:\/\/"
        + spec["routing"]["host"]
        + r"\/"
        + spec["routing"]["path"]
        + r"$",
        full_url,
    )
    assert re_match is not None


@pytest.fixture
def expected_templates():
    def _expected_templates(pvc_enabled):
        output = {
            "service": "service.yaml",
            "ingress": "ingress.yaml",
            "statefulset": "statefulset.yaml",
            "configmap": "configmap.yaml",
            "secret": "secret.yaml",
        }
        if pvc_enabled:
            return {**output, "pvc": "pvc.yaml"}
        else:
            return output

    yield _expected_templates


@pytest.mark.parametrize("pvc_enabled", [True, False])
def test_get_children_templates(pvc_enabled, expected_templates):
    templates = get_children_templates(pvc_enabled)
    expected_templates = expected_templates(pvc_enabled)
    assert templates == expected_templates


def test_create_template_values(valid_spec):
    expected_keys = [
        "auth",
        "authentication_plugin_cookie_secret",
        "cookie_allowlist",
        "cookie_blocklist",
        "full_url",
        "host_url",
        "ingress_annotations",
        "jupyter_server",
        "jupyter_server_app_token",
        "jupyter_server_cookie_secret",
        "name",
        "oidc",
        "path",
        "probe_path",
        "pvc",
        "routing",
        "scheduler_name",
        "storage",
    ]
    spec = valid_spec()
    template_values = create_template_values("test_name", spec)
    assert all([k in expected_keys for k in template_values.keys()])
