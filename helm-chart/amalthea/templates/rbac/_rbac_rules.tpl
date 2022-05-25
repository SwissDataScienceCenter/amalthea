{{- define "amalthea.rules" }}
  # Kopf: posting the events about the handlers progress/errors.
  - apiGroups: [""]
    resources: [events]
    verbs: [create]

  # Amalthea: watching & handling for the custom resource we declare.
  - apiGroups: [{{ .Values.crdApiGroup }}]
    resources: [{{ .Values.crdNames.plural }}]
    verbs: [get, list, watch, patch, delete]

  - apiGroups: [""]
    resources: [pods]
    verbs: [get, list, watch, delete]
  
  - apiGroups: [""]
    resources: [pods/exec]
    verbs: [create, get]

  # Amalthea get pod metrics used to cull idle Jupyter servers
  - apiGroups: ["metrics.k8s.io"]
    resources: [pods]
    verbs: [get, list, watch]

  # Amalthea: child resources we produce
  # Note that we do not patch/update/delete them ever.
  - apiGroups:
      - ""
      - apps
      - networking.k8s.io
    resources:
      - statefulsets
      - persistentvolumeclaims
      - services
      - ingresses
      - secrets
      - configmaps
    verbs: [create, get, list, watch]

    {{- range .Values.extraChildResources }}
  - apiGroups:
      - {{ .group }}
    resources:
      - {{ .name }}
    verbs: [create, get, list, watch]
    {{- end }}

{{- end }}
