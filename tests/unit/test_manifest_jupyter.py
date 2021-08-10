import pytest
import re
from controller.k8s_resources import get_children_specs
import logging


@pytest.mark.parametrize(
    "resources",
    [
        None,
        {
            "requests": {
                "memory": "1G",
                "cpu": "1",
            },
            "limits": {
                "memory": "2G",
                "cpu": "2",
            },
        },
    ],
)
def test_jupyterserver(resources, valid_spec):
    server = {
        "image": "test-image",
        "defaultUrl": "/url",
        "rootDir": "/root/dir",
    }
    if resources is not None:
        server["resources"] = resources
    spec = valid_spec(jupyterServer=server)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    js_container = manifest["statefulset"]["spec"]["template"]["spec"]["containers"][0]
    assert js_container["image"] == server["image"]
    assert (
        len(
            re.findall(
                re.escape("c.ServerApp.default_url=")
                + r"[\'\"]{1}"
                + re.escape(server["defaultUrl"]),
                manifest["configmap"]["data"]["jupyter_server_config.py"],
            )
        )
        == 1
    )
    assert (
        len(
            re.findall(
                re.escape("c.NotebookApp.default_url=")
                + r"[\'\"]{1}"
                + re.escape(server["defaultUrl"]),
                manifest["configmap"]["data"]["jupyter_notebook_config.py"],
            )
        )
        == 1
    )
    assert js_container["workingDir"] == server["rootDir"]
    if resources is not None:
        assert js_container["resources"] == resources
    else:
        assert js_container["resources"] == {}
