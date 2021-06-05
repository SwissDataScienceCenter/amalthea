{{- define "amalthea.rules" }}
rules:
  - apiGroups:
      - ""
      - apps
      - extensions
    resources:
      - statefulsets
      - pods
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
      - jupyterservers
    verbs:
      - get
      - list
      - watch
      - patch
{{- end }}
