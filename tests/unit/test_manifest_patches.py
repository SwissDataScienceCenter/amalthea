import pytest
from controller.k8s_resources import get_children_specs
import logging


@pytest.mark.parametrize(
    "patch",
    [
        {
            "type": "application/json-patch+json",
            "patch": [
                {
                    "op": "add",
                    "path": "/extra_pod",
                    "value": {
                        "apiVersion": "v1",
                        "kind": "Pod",
                        "metadata": {"name": "new_pod_name"},
                        "spec": {
                            "containers": [
                                {
                                    "name": "app",
                                    "image": "test_pod_image:latest",
                                }
                            ]
                        },
                    },
                }
            ],
        },
        {
            "type": "application/merge-patch+json",
            "patch": {
                "extra_pod": {
                    "apiVersion": "v1",
                    "kind": "Pod",
                    "metadata": {"name": "new_pod_name"},
                    "spec": {
                        "containers": [
                            {
                                "name": "app",
                                "image": "test_pod_image:latest",
                            }
                        ]
                    },
                }
            },
        },
    ],
)
def test_add_pod(patch, valid_spec):
    patches = [patch]
    spec = valid_spec(patches=patches)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    if patch["type"] == "application/json-patch+json":
        assert manifest["extra_pod"] == patch["patch"][0]["value"]
        assert len(manifest.keys()) > 1
    else:
        assert manifest["extra_pod"] == patch["patch"]["extra_pod"]
        assert len(manifest.keys()) > 1


@pytest.mark.parametrize(
    "patch",
    [
        {
            "type": "application/json-patch+json",
            "patch": [
                {
                    "op": "add",
                    "path": "/statefulset/spec/template/spec/containers/-",
                    "value": {
                        "name": "new_container",
                        "image": "test_container_image:latest",
                    },
                }
            ],
        }
    ],
)
def test_add_container(patch, valid_spec):
    patches = [patch]
    spec = valid_spec(patches=patches)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    assert (
        manifest["statefulset"]["spec"]["template"]["spec"]["containers"][-1]
        == patch["patch"][0]["value"]
    )


@pytest.mark.parametrize(
    "patch",
    [
        {
            "type": "application/json-patch+json",
            "patch": [
                {
                    "op": "add",
                    "path": "/image_pull_secret",
                    "value": {
                        "apiVersion": "v1",
                        "data": {".dockerconfigjson": "registry_secret"},
                        "kind": "Secret",
                        "metadata": {
                            "name": "image_pull_secret_name",
                        },
                        "type": "kubernetes.io/dockerconfigjson",
                    },
                },
                {
                    "op": "add",
                    "path": "/statefulset/spec/template/spec/imagePullSecrets/-",
                    "value": {"name": "image_pull_secret_name"},
                },
            ],
        },
        {
            "type": "application/merge-patch+json",
            "patch": {
                "image_pull_secret": {
                    "apiVersion": "v1",
                    "data": {".dockerconfigjson": "registry_secret"},
                    "kind": "Secret",
                    "metadata": {
                        "name": "image_pull_secret_name",
                    },
                    "type": "kubernetes.io/dockerconfigjson",
                },
                "statefulset": {
                    "spec": {
                        "template": {
                            "spec": {
                                "imagePullSecrets": [{"name": "image_pull_secret_name"}]
                            }
                        }
                    }
                },
            },
        },
    ],
)
def test_add_image_pull_secret(patch, valid_spec):
    patches = [patch]
    spec = valid_spec(patches=patches)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    if patch["type"] == "application/json-patch+json":
        assert manifest["image_pull_secret"] == patch["patch"][0]["value"]
        assert (
            manifest["statefulset"]["spec"]["template"]["spec"]["imagePullSecrets"][0]
            == patch["patch"][1]["value"]
        )
        assert len(manifest.keys()) > 1
    else:
        assert manifest["image_pull_secret"] == patch["patch"]["image_pull_secret"]
        assert (
            manifest["statefulset"]["spec"]["template"]["spec"]["imagePullSecrets"][0]
            == patch["patch"]["statefulset"]["spec"]["template"]["spec"][
                "imagePullSecrets"
            ][0]
        )
        assert len(manifest.keys()) > 1
