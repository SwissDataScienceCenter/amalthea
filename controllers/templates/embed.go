package templates

import "embed"

//go:embed configmap.yaml
//go:embed ingress.yaml
//go:embed pvc.yaml
//go:embed secret.yaml
//go:embed service.yaml
//go:embed statefulset.yaml
var Templates embed.FS
