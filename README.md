# iftop-exporter

## Deploy with Helm

```bash

$ helm repo add iftop-exporter git+https://github.com/bougou/iftop-exporter@deploy/charts?ref=v0.1.0
$ helm upgrade --install iftop-exporter -n kube-prometheus -f iftop-exporter/values.yaml iftop-exporter/iftop-exporter  --version=1.0.0
```
