{{- define "amalthea.rules" }}
  # Kopf: posting the events about the handlers progress/errors.
  - apiGroups: [""]
    resources: [events]
    verbs: [create, get, list, watch]

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
    resources:
      - persistentvolumeclaims
      - services
      - secrets
      - configmaps
    verbs: [create, get, list, watch]
  - apiGroups:
      - apps
    resources:
      - statefulsets
    verbs: [create, get, list, watch]
  - apiGroups:
      - networking.k8s.io
    resources:
      - ingresses
    verbs: [create, get, list, watch]

  # Required for hibernating sessions
  - apiGroups: ["apps"]
    resources: ["statefulsets"]
    verbs: [patch]

    {{- range .Values.extraChildResources }}
  - apiGroups:
      - {{ .group }}
    resources:
      - {{ .name }}
    verbs: [create, get, list, watch]
    {{- end }}

{{- end }}
