from io import StringIO
from pathlib import Path
from typing import Any

from yaml import safe_load


def jupyter_server_crd() -> str:
    """Read the jupyter server CRD manifest into a string."""
    parent_dir = Path(__file__).parent.resolve()
    with open(parent_dir / "jupyter_server.yaml") as f:
        return f.read()


def jupyter_server_crd_dict() -> dict[str, Any]:
    """Load the jupyter server CRD manifest as a dictionary."""
    return safe_load(StringIO(jupyter_server_crd()))
