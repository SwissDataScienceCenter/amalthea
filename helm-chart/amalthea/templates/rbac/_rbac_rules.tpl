{{- define "amalthea.rules" }}
  # Kopf: posting the events about the handlers progress/errors.
  - apiGroups: [""]
    resources: [events]
    verbs: [create]

  # Amalthea: watching & handling for the custom resource we declare.
  - apiGroups: [renku.io]
    resources: [jupyterservers]
    verbs: [list, watch, patch]

  - apiGroups: [""]
    resources: [pods]
    verbs: [get, list, watch, delete]

  # Amalthea: child resources we produce
  # Note that we do not patch/update/delete them ever.
  - apiGroups:
      - ""
      - apps
      - extensions
    {{- range .Values.rbac.extraChildApiGroups }}
      - {{ . }}
    {{- end }}
    resources:
      - statefulsets
      - persistentvolumeclaims
      - services
      - ingresses
      - secrets
      - configmaps
    {{- range .Values.rbac.extraChildResources }}
      - {{ . }}
    {{- end }}
    verbs: [create, get, list, watch]

{{- end }}
