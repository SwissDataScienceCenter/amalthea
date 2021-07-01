import pytest

from controller.src.server_controller import (
    get_labels,
    PARENT_NAME_LABEL_KEY,
    PARENT_UID_LABEL_KEY,
    CHILD_KEY_LABEL_KEY,
    MAIN_POD_LABEL_KEY,
)


@pytest.fixture
def parent_labels():
    yield {"label1key": "label1value"}


@pytest.fixture
def expected_label_keys():
    def _expected_label_keys(child_key, is_main_pod, parent_labels):
        keys = [
            "app.kubernetes.io/component",
            PARENT_UID_LABEL_KEY,
            PARENT_NAME_LABEL_KEY,
            *list(parent_labels.keys()),
        ]
        if child_key:
            keys.append(CHILD_KEY_LABEL_KEY)
        if is_main_pod:
            keys.append(MAIN_POD_LABEL_KEY)
        return keys

    yield _expected_label_keys


@pytest.mark.parametrize("child_key", [None, "child_key"])
@pytest.mark.parametrize("is_main_pod", [True, False])
def test_get_labels(is_main_pod, child_key, expected_label_keys, parent_labels):
    parent_name = "parent_name"
    parent_uid = "parent_uid"
    labels = get_labels(parent_name, parent_uid, parent_labels, child_key, is_main_pod)
    expected_keys = expected_label_keys(child_key, is_main_pod, parent_labels)
    assert all([k in expected_keys for k in labels.keys()])
