import yaml
import subprocess
from ruamel.yaml import YAML


yaml2 = YAML()

repo_root = (
    subprocess.check_output("git rev-parse --show-toplevel", shell=True)
    .decode()
    .strip()
)

rendered_chart = (
    subprocess.check_output(
        f"helm template {repo_root}/helm-chart/amalthea",
        shell=True,
    )
    .decode()
    .split("---")
)

for spec in rendered_chart:
    obj = yaml2.load(spec)
    if obj and obj["kind"] == "CustomResourceDefinition":
        with open((f"{repo_root}/docs/crd.yaml"), "w") as outfile:
            outfile.write(
                "# This CRD is automatically generated from the helm charts, do not modify! \n"
            )
            yaml2.dump(obj, outfile)
