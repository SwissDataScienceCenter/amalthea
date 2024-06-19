import sys

from difflib import unified_diff
from pathlib import Path

from controller.crds import jupyter_server_crd


def _template_plain(output: Path):
    latest_crd = jupyter_server_crd()
    with open(output, "w") as f:
        f.write("# This manifest is auto-generated from controller/crds/jupyter_server.yaml, do not modify.\n")
        f.write(latest_crd.strip())
        f.write("\n")


def _template_helm(output: Path):
    latest_crd = jupyter_server_crd()
    with open(output, "w") as f:
        f.write("{{- if .Values.deployCrd -}}\n")
        f.write("# This manifest is auto-generated from controller/crds/jupyter_server.yaml, do not modify.\n")
        f.write(latest_crd.strip())
        f.write("\n{{- end }}")
        f.write("\n")


def _template():
    root_dir = Path(__file__).parent.parent.parent.resolve()
    _template_helm(root_dir / "helm-chart" / "amalthea" / "templates" / "crd.yaml")
    _template_plain(root_dir / "manifests" / "crd.yaml")


def _diff_path(output: Path, skip_headers: int = 0, skip_footers: int = 0) -> str:
    latest_spec = jupyter_server_crd().strip()
    with open(output) as f:
        template = f.read()
    lines_to_diff = template.strip().splitlines()[skip_headers : -skip_footers if skip_footers > 0 else None]
    diff_lines = list(
        unified_diff(
            lines_to_diff,
            latest_spec.splitlines(),
            output.as_posix(),
            output.as_posix(),
        )
    )
    return "\n".join(diff_lines)


def _check():
    root_dir = Path(__file__).parent.parent.parent.resolve()
    diffs: list[str] = []
    diffs.append(_diff_path(root_dir / "helm-chart" / "amalthea" / "templates" / "crd.yaml", 2, 1))
    diffs.append(_diff_path(root_dir / "manifests" / "crd.yaml", 1))
    if any([len(diff) > 0 for diff in diffs]):
        diffs_str = "\n\n".join(diffs)
        raise Exception(f"Some CRD templates are out of date:\n{diffs_str}")


if __name__ == "__main__":
    match sys.argv:
        case [_, "template"]:
            _template()
        case [_, "check"]:
            _check()
        case _:
            raise ValueError(f"Received unknown arguments {sys.argv}")
