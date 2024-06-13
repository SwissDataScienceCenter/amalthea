from expiringdict import ExpiringDict
from kubernetes.client.rest import ApiException
from kubernetes import config as k8s_config, dynamic
from kubernetes.client import api_client
from kubernetes.client.api import core_v1_api
from kubernetes.stream import stream
import logging
import re

# A very simple in-memory cache to store the result of the
# "resources" query of the dynamic API client.
api_cache = ExpiringDict(max_len=100, max_age_seconds=60)


def get_pod_metrics(pod_name, namespace):
    """
    Get the resource usage from the k8s metrics api for a specific pod.
    """
    k8s_config.load_config()
    client = dynamic.DynamicClient(api_client.ApiClient())
    try:
        res = client.request(
            "GET",
            f"/apis/metrics.k8s.io/v1beta1/namespaces/{namespace}/pods/{pod_name}",
        )
    except ApiException as err:
        logging.warning(f"Could not get metrics for pod {pod_name} " f"in namespace {namespace}, because: {type(err)}")
        return None

    return res


def parse_pod_metrics(metrics):
    """
    Parse the response from the k8s metrics api. Core usage is converted from nanocores
    to millicores and memory usage from Kibibytes (Ki) to bytes.
    """
    parsed_metrics = []
    try:
        containers = metrics.containers
    except AttributeError:
        containers = []
    for container in containers:
        try:
            parsed_data = {"name": container.name}
            parsed_data["cpu_millicores"] = convert_to_millicores(container["usage"]["cpu"])
            parsed_data["memory_bytes"] = convert_to_bytes(container["usage"]["memory"])
            parsed_metrics.append(parsed_data)
        except (KeyError, ValueError, AttributeError) as err:
            logging.warning(f"Could not parse metrics {metrics} because: {err}")
    return parsed_metrics


def get_volume_disk_capacity(pod_name, namespace, volume_name):
    """
    Find the container in the specified pod that has a volume named
    `volume_name` and run df -h or du -sb in that container to determine
    the available space in the volume.
    """
    api = get_api("v1", "Pod")
    res = api.get(name=pod_name, namespace=namespace)
    if res.kind == "PodList":
        # make sure there is only one pod with the requested name
        if len(res.items) == 1:
            pod = res.items[0]
        else:
            return {}
    else:
        pod = res

    containers = pod.spec.get("initContainers", []) + pod.spec.get("containers", [])
    for container in containers:
        for volume_mount in container.get("volumeMounts", []):
            if volume_mount.get("name") == volume_name:
                mount_path = volume_mount.get("mountPath")
                volume = list(filter(lambda x: x.name == volume_name, pod.spec.volumes))
                volume = volume[0] if len(volume) == 1 else {}
                if "emptyDir" in volume.keys() and volume["emptyDir"].get("sizeLimit") is not None:
                    # empty dir is used for the session
                    command = ["sh", "-c", f"du -sb {mount_path}"]
                    used_bytes = parse_du_command(
                        pod_exec(
                            pod_name,
                            namespace,
                            container.name,
                            command,
                        )
                    )
                    total_bytes = convert_to_bytes(volume["emptyDir"]["sizeLimit"])
                    available_bytes = 0 if total_bytes - used_bytes < 0 else total_bytes - used_bytes
                    return {
                        "total_bytes": total_bytes,
                        "used_bytes": used_bytes,
                        "available_bytes": available_bytes,
                    }
                else:
                    # PVC is used for the session
                    command = ["sh", "-c", f"df -Pk {mount_path}"]
                    try:
                        disk_cap_raw = pod_exec(
                            pod_name,
                            namespace,
                            container.name,
                            command,
                        )
                    except ApiException:
                        disk_cap_raw = ""
                        logging.warning(
                            f"Checking disk capacity failed with {pod_name}, "
                            f"{namespace}, {container.name}, {command}."
                        )
                    else:
                        logging.info(
                            f"Checking disk capacity succeeded with {pod_name}, "
                            f"{namespace}, {container.name}, {command}."
                        )
                    disk_cap = parse_df_command(disk_cap_raw)
                    # make sure `df -h` returned the results from only one mount point
                    if len(disk_cap) == 1:
                        return disk_cap[0]
    return {}


def pod_exec(pod_name, namespace, container_name, command):
    """
    Execute the specific command (list of strings)
    in the speicific namespace/pod/container and return the results.
    """
    k8s_config.load_config()
    api = core_v1_api.CoreV1Api()
    resp = stream(
        api.connect_get_namespaced_pod_exec,
        pod_name,
        namespace,
        command=command,
        container=container_name,
        stderr=True,
        stdin=False,
        stdout=True,
        tty=False,
        _preload_content=True,
    )
    return resp


def parse_df_command(capacity, bytes_multiplier=1024):
    """
    Parse the metrics from running `df -h` in a container.
    """
    output = []
    lines = capacity.strip("\n").split("\n")
    if not lines[0].startswith("Filesystem") or "Used" not in lines[0] or "Available" not in lines[0]:
        return output
    else:
        lines[0] = lines[0].replace("Mounted on", "Mounted_on")
        header = re.split(r"\s+", lines[0])

    for line in lines[1:]:
        data = dict(zip(header, re.split(r"\s+", line)))
        data["used_bytes"] = float(data["Used"]) * bytes_multiplier
        data["available_bytes"] = float(data["Available"]) * bytes_multiplier
        data["total_bytes"] = data["used_bytes"] + data["available_bytes"]
        output.append(data)

    return output


def parse_du_command(capacity, bytes_multiplier=1024):
    """
    Parse the result from running `du -sb` in a container.
    """
    try:
        return float(capacity.split()[0]) * bytes_multiplier
    except (KeyError, ValueError) as e:
        logging.warning(f"Could not parse du command because: {e}")
    return None


def get_api(api_version, kind, group=None):
    """
    Get the proper API for a certain resource. We cache the resources
    available in the cluster for 60 seconds in order to reduce the amount
    of unnecessary requests in busy clusters.
    """
    try:
        return api_cache[(api_version, kind, group)]
    except KeyError:
        client = dynamic.DynamicClient(api_client.ApiClient())
        api_cache[(api_version, kind, group)] = client.resources.get(
            api_version=api_version,
            kind=kind,
            group=group,
        )
        return api_cache[(api_version, kind, group)]


def convert_to_bytes(value):
    """
    Convert values from k8s like Ki,K,M,G,Gi etc to bytes
    """
    factors = {
        "K": 1000,
        "M": 1000**2,
        "G": 1000**3,
        "T": 1000**4,
        "P": 1000**5,
        "E": 1000**6,
        "Ki": 1024,
        "Mi": 1024**2,
        "Gi": 1024**3,
        "Ti": 1024**4,
        "Pi": 1024**5,
        "Ei": 1024**6,
    }
    res = re.match(r"^(?<!-)([0-9]*\.?[0-9]*)((?<=[0-9.])[EPTGMKi]*)$", str(value).strip())
    if res is None:
        raise ValueError(f"Cannot convert value {value} to bytes.")
    value, unit = res.groups()
    if unit and unit not in factors.keys():
        raise ValueError(f"Cannot convert value {value} to bytes because unit {unit} is not known.")
    return float(value) * (factors[unit] if unit else 1)


def convert_to_millicores(value):
    """
    Convert values from k8s like 1, 100m, 1000000n etc to millicores
    """
    factors = {
        "m": 1,
        "n": 1e-6,
    }
    res = re.match(r"^(?<!-)([0-9]*\.?[0-9]*)([mn]?)$", str(value).strip())
    if res is None:
        raise ValueError(f"Cannot convert value {value} to millicores.")
    value, unit = res.groups()
    if unit and unit not in factors.keys():
        raise ValueError(f"Cannot convert value {value} to millicores because unit {unit} is not known.")
    return float(value) * (factors[unit] if unit else 1000)
