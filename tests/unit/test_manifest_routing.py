import pytest
from controller.k8s_resources import get_children_specs
import logging


@pytest.mark.parametrize(
    "tls",
    [
        {"enabled": True, "secretName": "tlsSecretName"},
        {
            "enabled": False,
        },
    ],
)
def test_routing(tls, valid_spec):
    routing = {
        "host": "session.host.com",
        "path": "/",
        "tls": tls,
        "ingressAnnotations": {
            "annotation1key": "annotation1value",
            "annotation2key": "annotation2value",
        },
    }
    spec = valid_spec(routing=routing)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    assert "ingress" in manifest.keys()
    ingress = manifest["ingress"]
    if tls["enabled"]:
        assert "tls" in ingress["spec"].keys()
        assert ingress["spec"]["tls"] == [{"hosts": [routing["host"]], "secretName": routing["tls"]["secretName"]}]
    else:
        assert "tls" not in ingress["spec"].keys()
    assert "rules" in ingress["spec"].keys()
    assert ingress["spec"]["rules"] == [
        {
            "host": routing["host"],
            "http": {
                "paths": [
                    {
                        "path": routing["path"],
                        "pathType": "Prefix",
                        "backend": {
                            "service": {
                                "name": name,
                                "port": {"number": 80},
                            },
                        },
                    },
                ],
            },
        }
    ]
