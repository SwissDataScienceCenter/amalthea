import pytest
import re
from io import StringIO
import yaml
from jsonschema import validate


@pytest.fixture
def default_spec(crd_schema):
    spec = {
        "auth": {
            "token": "token",
            "oidc": {"enabled": False},
            "cookieAllowlist": [],
        },
        "jupyterServer": {
            "defaultUrl": "default_url",
            "image": "jupyter/minimal-notebook:latest",
            "rootDir": "/home/jovyan/work/",
            "resources": {},
        },
        "routing": {
            "host": "test.host",
            "path": "test_path",
            "tls": {"enabled": False},
            "ingressAnnotations": {},
        },
        "storage": {"size": "1G", "pvc": {"enabled": False}},
        "patches": [],
    }
    # check spec passes against crd schema
    validate(spec, crd_schema)
    return spec


@pytest.fixture
def valid_spec(default_spec, crd_schema):
    def _valid_spec(**kwargs):
        spec = {**default_spec, **kwargs}
        # check spec passes against crd schema
        validate(spec, crd_schema)
        return spec

    yield _valid_spec


@pytest.fixture(scope="session", autouse=True)
def crd_schema():
    crd_loc = "helm-chart/amalthea/templates/crd.yaml"
    with open(crd_loc, "r") as f:
        lines = f.readlines()
    start_ind = 0
    for i, l in enumerate(lines):
        if re.match(r"^\s*openAPIV3Schema:\s*$", l) is not None:
            start_ind = i
    lines = lines[start_ind + 1 : -1]
    f = StringIO("".join(lines))
    schema = yaml.safe_load(f)["properties"]["spec"]
    return schema
