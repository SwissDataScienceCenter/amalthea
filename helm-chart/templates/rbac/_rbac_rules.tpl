{{- define "jupyter-server-operator.rules" }}
rules:
  - apiGroups:
      - ""
      - apps
      - extensions
    resources:
      - statefulsets
      - ingresses
      - services
      - secrets
      - configmaps
      - persistentvolumeclaims
      - events
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - patch
  - apiGroups:
      - "renku.io"
    resources:
      - jupyterserver
    verbs:
      - get
      - list
      - watch
      - patch
{{- end }}
