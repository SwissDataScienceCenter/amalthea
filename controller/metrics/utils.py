from dataclasses import dataclass
import logging
from typing import Any, Dict, Optional

from controller.utils import convert_to_bytes, convert_to_millicores


@dataclass
class ResourceRequest:
    cpu_millicores: float
    memory_bytes: float
    disk_bytes: float
    gpus: int = 0


def resource_request_from_manifest(manifest: Dict[str, Any]) -> Optional[ResourceRequest]:
    resources = manifest.get("spec", {}).get("jupyterServer", {}).get("resources", {}).get(
        "requests", {}
    )
    disk_request = manifest.get("spec", {}).get("storage", {}).get("size")
    if disk_request:
        resources["disk_request"] = disk_request
    resource_name_xref = {
        "cpu": "cpu_millicores",
        "memory": "memory_bytes",
        "nvidia.com/gpu": "gpus",
    }
    resource_value_converters = {
        "cpu": convert_to_millicores,
        "memory": convert_to_bytes,
        "nvidia.com/gpu": lambda x: int(x),
        "disk_request": convert_to_bytes,
    }
    resources_parsed = {}
    for resource, resource_value in resources.items():
        parsed_resource_name = resource_name_xref.get(resource)
        if not parsed_resource_name:
            continue
        value_converter = resource_value_converters.get(resource)
        if not value_converter:
            continue
        try:
            resources_parsed[parsed_resource_name] = value_converter(resource_value)
        except ValueError:
            logging.warning(
                f"Could not covert the metric value {resource_value} "
                f"for resource {resource} with converter {value_converter}"
            )
    try:
        return ResourceRequest(**resources_parsed)
    except TypeError:
        return None
