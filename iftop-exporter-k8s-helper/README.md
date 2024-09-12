# iftop-exporter-k8s-helper

The `iftop-exporter-k8s-helper` is a kubebuilder based project. It
- watches the pods by specified `selectors` (use labels to match target pods),
- and then fetch the node-side `interface` name of the pod/container,
- and then write a file with `interface` name to the dynamic-dir of `iftop-exporter`.

Then, the `iftop-exporter` can start `iftop` programs for those dynamic found interfaces, and collect and interpret the metrics.

## Why `iftop-exporter-k8s-helper` exists

In most cases, you don't want to start one `iftop` program for each of all the pods in the K8S cluster.

The `iftop` program needs to known the interface name. So we need to dynamically fetch the interface names of the pods that we cared.

The `iftop-exporter` accepts static interface names and also watches a dynamic dir. It would be notified if any file operations occurred in the dynamic dir through [fsnotify](https://github.com/fsnotify/fsnotify).

You can just put files named with the names of the interface into the dynamic dir, then `iftop-exporter` would receive the information and start `iftop` programs for these interfaces.
