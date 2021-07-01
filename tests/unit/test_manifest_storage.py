import pytest
from controller.src.server_controller import get_children_specs
import logging


@pytest.mark.parametrize("pvc_enabled", [True, False])
@pytest.mark.parametrize("size", ["1G", "10G"])
@pytest.mark.parametrize("storage_class", ["standard"])
def test_storage(storage_class, size, pvc_enabled, valid_spec):
    storage = {
        "size": size,
        "pvc": {
            "enabled": pvc_enabled,
            "storageClassName": storage_class,
        },
    }
    spec = valid_spec(storage=storage)
    name = "test"
    manifest = get_children_specs(name, spec, logging)
    volumes = manifest["statefulset"]["spec"]["template"]["spec"]["volumes"]
    volume_mounts = manifest["statefulset"]["spec"]["template"]["spec"]["containers"][
        0
    ]["volumeMounts"]
    if pvc_enabled:
        assert {"name": "workspace", "emptyDir": {"sizeLimit": size}} not in volumes
        assert "pvc" in manifest.keys()
        assert manifest["pvc"] == {
            "kind": "PersistentVolumeClaim",
            "apiVersion": "v1",
            "metadata": {"name": name},
            "spec": {
                "accessModes": ["ReadWriteOnce"],
                "resources": {"requests": {"storage": size}},
                "storageClassName": storage_class,
            },
        }
        assert {
            "name": "workspace",
            "persistentVolumeClaim": {"claimName": name},
        } in volumes
    else:
        assert {"name": "workspace", "emptyDir": {"sizeLimit": size}} in volumes
        assert "pvc" not in manifest.keys()
    assert {
        "name": "workspace",
        "mountPath": "/home/jovyan/work/",
        "subPath": "work",
    } in volume_mounts
