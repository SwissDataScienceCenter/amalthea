import pytest
from controller.k8s_resources import get_children_specs
import logging
from base64 import b64encode


@pytest.mark.parametrize("cookie_allow_list", [[], ["allow_cookie1", "allow_cookie2"]])
@pytest.mark.parametrize("cookie_block_list", [[], ["block_cookie1", "block_cookie2"]])
@pytest.mark.parametrize("token", ["", None, "secret_token"])
def test_auth_no_oidc(token, cookie_block_list, cookie_allow_list, valid_spec):
    auth = {
        "oidc": {
            "enabled": False,
        },
    }
    if token is not None:
        auth["token"] = token
    spec = valid_spec(auth=auth)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    secret = manifest["secret"]
    js_container = manifest["statefulset"]["spec"]["template"]["spec"]["containers"][0]
    assert "jupyterServerAppToken" in secret["data"].keys()
    assert {
        "name": "SERVER_APP_TOKEN",
        "valueFrom": {
            "secretKeyRef": {
                "name": name,
                "key": "jupyterServerAppToken",
            },
        },
    } in js_container["env"]
    if token == "":
        assert secret["data"]["jupyterServerAppToken"] is None
    elif token is None:
        assert secret["data"]["jupyterServerAppToken"] is not None
    else:
        assert secret["data"]["jupyterServerAppToken"] == b64encode(token.encode()).decode()
    assert not any(
        [
            container["name"] in ["authentication-plugin", "authorization-plugin"]
            for container in manifest["statefulset"]["spec"]["template"]["spec"]["containers"]
        ]
    )


@pytest.mark.parametrize(
    "oidc_secret",
    [
        {"value": "oidc_secret"},
        {"secretKeyRef": {"name": "secret_name", "key": "secret_key"}},
    ],
)
def test_auth_oidc(oidc_secret, valid_spec):
    auth = {
        "oidc": {
            "enabled": True,
            "issuerUrl": "https://issuer.url",
            "clientId": "clientId",
            "clientSecret": oidc_secret,
        },
    }
    spec = valid_spec(auth=auth)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    secret = manifest["secret"]
    assert any(
        [
            container["name"] == "oauth2-proxy"
            for container in manifest["statefulset"]["spec"]["template"]["spec"]["containers"]
        ]
    )
    assert "oauth2ProxyCookieSecret" in secret["data"].keys()
    auth_container = manifest["statefulset"]["spec"]["template"]["spec"]["containers"][1]
    auth_container_oidc_secret = [env for env in auth_container["env"] if env["name"] == "OAUTH2_PROXY_CLIENT_SECRET"][
        0
    ]
    if "value" in oidc_secret.keys():
        assert "oidcClientSecret" in secret["data"].keys()
        assert secret["data"]["oidcClientSecret"] == b64encode(oidc_secret["value"].encode()).decode()
        assert auth_container_oidc_secret["valueFrom"]["secretKeyRef"] == {
            "key": "oidcClientSecret",
            "name": name,
        }
    else:
        assert auth_container_oidc_secret["valueFrom"]["secretKeyRef"] == oidc_secret["secretKeyRef"]
