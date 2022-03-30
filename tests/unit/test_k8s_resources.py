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
    def _expected_templates(template_type, pvc_enabled):
        output = {
            "service": f"{template_type}/service.yaml",
            "ingress": f"{template_type}/ingress.yaml",
            "statefulset": f"{template_type}/statefulset.yaml",
            "configmap": f"{template_type}/configmap.yaml",
            "configmap-proxy": f"{template_type}/configmap-proxy.yaml",
            "secret": f"{template_type}/secret.yaml",
        }
        if pvc_enabled:
            return {**output, "pvc": f"{template_type}/pvc.yaml"}
        else:
            return output

    yield _expected_templates


@pytest.mark.parametrize("pvc_enabled", [True, False])
def test_get_children_templates(pvc_enabled, expected_templates):
    templates = get_children_templates("jupyterlab", pvc_enabled)
    expected_templates = expected_templates("jupyterlab", pvc_enabled)
    assert templates == expected_templates


def test_create_template_values(valid_spec):
    expected_keys = [
        "auth",
        "authentication_plugin_cookie_secret",
        "full_url",
        "host_url",
        "ingress_annotations",
        "jupyter_server",
        "cookie_secret",
        "name",
        "oidc",
        "basic_auth",
        "path",
        "pvc",
        "routing",
        "scheduler_name",
        "storage",
    ]
    spec = valid_spec()
    template_values = create_template_values("test_name", spec)
    assert all([k in expected_keys for k in template_values.keys()])
