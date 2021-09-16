import yaml
import subprocess

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
    obj = yaml.safe_load(spec)
    if obj and obj["kind"] == "CustomResourceDefinition":
        with open((f"{repo_root}/docs/crd.yaml"), "w") as outfile:
            outfile.write(
                "# This CRD is automatically generated from the helm charts, do not modify! \n"
            )
            yaml.safe_dump(obj, outfile)
