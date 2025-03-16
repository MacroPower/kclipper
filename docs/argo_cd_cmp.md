# Argo CD Config Management Plugin

You can use the below Argo CD values to add a Config Management Plugin for kclipper:

```yaml
configs:
  cmp:
    create: true
    plugins:
      kcl:
        generate:
          command: [kcl]
          args:
            - run
            - --no_style
            - --quiet
            - --log_level=info
            - --log_format=json
        discover:
          fileName: "*.k"

repoServer:
  extraContainers:
    - name: kcl-plugin
      image: ghcr.io/macropower/kclipper:latest
      command: [/var/run/argocd/argocd-cmp-server]
      securityContext:
        runAsNonRoot: true
        runAsUser: 999
        runAsGroup: 999
      volumeMounts:
        - name: var-files
          mountPath: /var/run/argocd
        - name: plugins
          mountPath: /home/argocd/cmp-server/plugins
        - name: kcl-plugin-config
          mountPath: /home/argocd/cmp-server/config/plugin.yaml
          subPath: kcl.yaml
        - name: cmp-tmp
          mountPath: /tmp
      # Add additional environment variables for KCL, KPM, etc., as needed.
      env:
        - name: KPM_FEATURE_GATES
          value: SupportMVS=true
        - name: KCL_FAST_EVAL
          value: "1"
  volumes:
    - name: kcl-plugin-config
      configMap:
        name: argocd-cmp-cm
    - name: cmp-tmp
      emptyDir: {}
```

[This guide](https://www.kcl-lang.io/docs/user_docs/guides/gitops/gitops-quick-start) from the KCL authors goes into more detail about integrating KCL with Argo CD.
