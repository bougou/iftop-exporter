# iftop-exporter


## Deploy with Helm

`iftop-exporter` is deployed as a DaemonSet with `hostNetwork=true` network mode in K8S environment.
`iftop-exporter-k8s-helper` is deployed as sidecar of `iftop-exporter`.
To avoid port conflicts, you can change the ports with custom values.

To select which pods/interfaces you want to monitor, you must specify `selectors` to filter out the pods.

```bash
$ vim values.yaml

exporter:
  port: "9999"

helper:
  # selectors:
  # - selector1Name:label1key,label2key==some-value
  # - selector2Name:label3key!=some-value,label4key==some-value
  #
  # Note:
  # 1. For each selector, the selectorName is MUST, the selectorName and labels
  #    are colon-separated (`:`), the labels are comma-separated (`,`).
  # 2. In each selector, the label selections are logical AND-ed, which means that
  #    all label selections MUST be all matched, then the selector is matched.
  #    For label selection,
  #    - Only "=", "==", "!=" are valid labelOperators. "=" and "==" have same result.
  #      - If label-value is omitted, it would be set to empty string.
  #      - If label-operator (and label-value) is omitted, it means to check the existence of the label key.
  # 3. Different selectors are logical OR-ed (selector1 or selector2), which means that
  #    any one selector is matched, the Pod is selected.
  selectors: []

  manager:
    logLevel: 1
    metricsPort: 8080
    healthPort: 8081

  # kube-rbac-proxy container
  # would proxy 8443 port to manager metricsPort 8080
  proxy:
    enabled: true
    port: 8443
```

```bash
$ helm repo add iftop-exporter git+https://github.com/bougou/iftop-exporter@deploy/charts?ref=v0.1.0
$ helm upgrade --install iftop-exporter -n kube-prometheus -f values.yaml iftop-exporter/iftop-exporter  --version=1.0.0
```
