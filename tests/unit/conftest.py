import pytest


@pytest.fixture
def default_spec():
    spec = {
        "auth": {
            "token": "token",
            "oidc": {"enabled": False},
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
        "type": "jupyterlab",
    }
    return spec


@pytest.fixture
def valid_spec(default_spec):
    def _valid_spec(**kwargs):
        spec = {**default_spec, **kwargs}
        return spec

    yield _valid_spec
