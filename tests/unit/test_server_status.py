import json
from typing import Any, Dict, List, Optional, Union

import pytest
import yaml

from controller.server_status import (
    ContainerStatus,
    ContainerTypeEnum,
    K8sPodPhaseEnum,
    ServerStatus,
)
from controller.server_status_enum import ServerStatusEnum


def get_failing_container_status(restart_count=10, exit_code=1):
    return yaml.safe_load(
        f"""
containerID: container-id
image: image
imageID: image-id
lastState:
    terminated:
        containerID: container-id
        exitCode: {exit_code}
        finishedAt: "2022-10-10T15:48:26Z"
        reason: Error
        startedAt: "2022-10-10T15:48:26Z"
name: container-name
ready: false
restartCount: {restart_count}
started: false
state:
    waiting:
        message: "Restarting failed container"
        reason: CrashLoopBackOff
        """
    )


def get_completed_init_container_status():
    return yaml.safe_load(
        """
containerID: container-id
image: image
imageID: image-id
lastState: {}
name: container-name
ready: true
restartCount: 0
state:
    terminated:
        containerID: container-id
        exitCode: 0
        finishedAt: "2022-10-10T15:48:02Z"
        reason: Completed
        startedAt: "2022-10-10T15:48:01Z"
        """
    )


def get_waiting_container():
    return yaml.safe_load(
        """
image: image-name
imageID: ""
lastState: {}
name: container-name
ready: false
restartCount: 0
started: false
state:
    waiting:
        reason: PodInitializing
        """
    )


def get_running_container_status(restart_count=0):
    return yaml.safe_load(
        f"""
containerID: container-id
image: image
imageID: image-id
lastState: {{}}
name: container-name
ready: true
restartCount: {restart_count}
started: true
state:
    running:
        startedAt: "2022-10-09T19:33:52Z"
        """
    )


def get_ok_pod_conditions():
    return yaml.safe_load(
        """
- lastProbeTime: null
  lastTransitionTime: "2022-10-09T19:33:51Z"
  status: "True"
  type: Initialized
- lastProbeTime: null
  lastTransitionTime: "2022-10-09T19:33:57Z"
  status: "True"
  type: Ready
- lastProbeTime: null
  lastTransitionTime: "2022-10-09T19:33:57Z"
  status: "True"
  type: ContainersReady
- lastProbeTime: null
  lastTransitionTime: "2022-10-09T19:33:29Z"
  status: "True"
  type: PodScheduled
        """
    )


def get_bad_pod_conditions():
    return yaml.safe_load(
        """
- lastProbeTime: null
  lastTransitionTime: "2022-10-10T15:59:29Z"
  message: 'containers with incomplete status: [init-certificates git-clone]'
  reason: ContainersNotInitialized
  status: "False"
  type: Initialized
- lastProbeTime: null
  lastTransitionTime: "2022-10-10T15:59:29Z"
  message: 'containers with unready status: [jupyter-server oauth2-proxy git-proxy
  git-sidecar]'
  reason: ContainersNotReady
  status: "False"
  type: Ready
- lastProbeTime: null
  lastTransitionTime: "2022-10-10T15:59:29Z"
  message: 'containers with unready status: [jupyter-server oauth2-proxy git-proxy
  git-sidecar]'
  reason: ContainersNotReady
  status: "False"
  type: ContainersReady
- lastProbeTime: null
  lastTransitionTime: "2022-10-10T15:59:29Z"
  status: "True"
  type: PodScheduled
        """
    )


def get_partial_manifest(
    conditions: List[Dict[str, Optional[Union[str, int, bool, float]]]] = [],
    init_container_statuses: List[Dict[str, Any]] = [],
    container_statuses: List[Dict[str, Any]] = [],
    phase: K8sPodPhaseEnum = K8sPodPhaseEnum.unknown,
) -> Dict[str, Any]:
    return yaml.safe_load(
        f"""
status:
    mainPod:
        status:
            phase: {phase.value}
            conditions: {json.dumps(conditions)}
            initContainerStatuses: {json.dumps(init_container_statuses)}
            containerStatuses: {json.dumps(container_statuses)}
        """
    )


@pytest.mark.parametrize(
    "container_status_dict,container_type,property_name,expected_value",
    [
        (get_waiting_container(), ContainerTypeEnum.regular, "ready", False),
        (get_waiting_container(), ContainerTypeEnum.regular, "running", False),
        (get_waiting_container(), ContainerTypeEnum.regular, "running_ready", False),
        (
            get_waiting_container(),
            ContainerTypeEnum.regular,
            "completed_successfully",
            False,
        ),
        (get_running_container_status(), ContainerTypeEnum.regular, "ready", True),
        (get_running_container_status(), ContainerTypeEnum.regular, "running", True),
        (
            get_running_container_status(),
            ContainerTypeEnum.regular,
            "running_ready",
            True,
        ),
        (
            get_running_container_status(),
            ContainerTypeEnum.regular,
            "completed_successfully",
            False,
        ),
        (get_completed_init_container_status(), ContainerTypeEnum.init, "ready", True),
        (
            get_completed_init_container_status(),
            ContainerTypeEnum.init,
            "running",
            False,
        ),
        (
            get_completed_init_container_status(),
            ContainerTypeEnum.init,
            "running_ready",
            False,
        ),
        (
            get_completed_init_container_status(),
            ContainerTypeEnum.init,
            "completed_successfully",
            True,
        ),
        (get_failing_container_status(), ContainerTypeEnum.regular, "ready", False),
        (get_failing_container_status(), ContainerTypeEnum.regular, "running", False),
        (
            get_failing_container_status(),
            ContainerTypeEnum.regular,
            "running_ready",
            False,
        ),
        (
            get_failing_container_status(),
            ContainerTypeEnum.regular,
            "completed_successfully",
            False,
        ),
    ],
)
def test_container_status(container_status_dict, container_type, property_name, expected_value):
    status = ContainerStatus.from_k8s_container_status(
        container_status_dict, container_type=container_type
    )
    assert getattr(status, property_name) == expected_value


@pytest.mark.parametrize(
    "manifest,expected_status",
    [
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [get_waiting_container()],
                [get_waiting_container()],
                K8sPodPhaseEnum.pending,
            ),
            ServerStatusEnum.Starting,
        ),
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [get_waiting_container(), get_running_container_status()],
                [get_waiting_container()],
                K8sPodPhaseEnum.pending,
            ),
            ServerStatusEnum.Starting,
        ),
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [get_running_container_status()],
                [get_waiting_container()],
                K8sPodPhaseEnum.pending,
            ),
            ServerStatusEnum.Starting,
        ),
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [get_completed_init_container_status()],
                [get_waiting_container()],
                K8sPodPhaseEnum.pending,
            ),
            ServerStatusEnum.Starting,
        ),
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [get_completed_init_container_status()],
                [get_waiting_container(), get_running_container_status()],
                K8sPodPhaseEnum.pending,
            ),
            ServerStatusEnum.Starting,
        ),
        (
            get_partial_manifest(
                get_bad_pod_conditions(),
                [get_completed_init_container_status()],
                [get_failing_container_status()],
                K8sPodPhaseEnum.failed,
            ),
            ServerStatusEnum.Failed,
        ),
        (
            get_partial_manifest(
                get_bad_pod_conditions(),
                [get_completed_init_container_status(), get_failing_container_status()],
                [get_failing_container_status()],
                K8sPodPhaseEnum.failed,
            ),
            ServerStatusEnum.Failed,
        ),
        (
            get_partial_manifest(
                get_bad_pod_conditions(),
                [get_failing_container_status()],
                [get_waiting_container(), get_failing_container_status()],
                K8sPodPhaseEnum.failed,
            ),
            ServerStatusEnum.Failed,
        ),
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [get_completed_init_container_status()],
                [get_running_container_status()],
                K8sPodPhaseEnum.running,
            ),
            ServerStatusEnum.Running,
        ),
        (
            get_partial_manifest(
                get_ok_pod_conditions(),
                [],
                [get_running_container_status()],
                K8sPodPhaseEnum.running,
            ),
            ServerStatusEnum.Running,
        ),
    ],
)
def test_server_status_from_server_manifest(manifest, expected_status, mocker):
    mocker.patch.object(
        ServerStatus,
        "server_url_is_eventually_responsive",
        return_value=True,
        autospec=True,
    )
    server = ServerStatus.from_server_spec(
        manifest,
        init_container_restart_limit=1,
        container_restart_limit=1,
    )
    assert server.overall_status == expected_status


def test_server_status_is_starting_if_url_check_fails(mocker):
    mocker.patch.object(
        ServerStatus,
        "server_url_is_eventually_responsive",
        return_value=False,
        autospec=True,
    )
    manifest = get_partial_manifest(
        get_ok_pod_conditions(),
        [get_completed_init_container_status()],
        [get_running_container_status()],
        K8sPodPhaseEnum.running,
    )
    server = ServerStatus.from_server_spec(
        manifest,
        init_container_restart_limit=1,
        container_restart_limit=1,
    )
    assert server.overall_status == ServerStatusEnum.Starting
