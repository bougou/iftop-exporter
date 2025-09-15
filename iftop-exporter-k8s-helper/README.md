# iftop-exporter-k8s-helper

`iftop-exporter-k8s-helper` is a Kubernetes controller built with [kubebuilder](https://book.kubebuilder.io/) for dynamically discovering and monitoring network interfaces of specific Pods in Kubernetes clusters.

## Overview

This component serves as an auxiliary tool for `iftop-exporter` and works by:

1. **Monitoring Pod Status**: Watches Pods that match specified label selectors
2. **Extracting Network Interface Information**: Retrieves node-side network interface names for Pod containers
3. **Dynamic Configuration**: Writes interface information to the dynamic directory of `iftop-exporter`
4. **Automatic Cleanup**: Automatically cleans up related interface files when Pods are deleted

## Architecture

```
┌─────────────────────┐    ┌──────────────────────┐    ┌─────────────────────┐
│   Kubernetes API    │    │  iftop-exporter-     │    │   iftop-exporter    │
│                     │◄───┤  k8s-helper          │───►│                     │
│  - Pod Events       │    │                      │    │  - fsnotify         │
│  - Label Selectors  │    │  - Controller        │    │  - iftop processes  │
└─────────────────────┘    │  - Interface Mapping │    │  - Metrics Export   │
                           └──────────────────────┘    └─────────────────────┘
```

## Core Features

### 1. Intelligent Pod Selection
- Supports flexible selector configuration based on labels
- Supports multiple selectors with logical OR relationship
- Label conditions within each selector use logical AND relationship

### 2. Interface Mapping
- Automatically retrieves container-to-node network interface mapping
- Supports interface discovery for multi-container Pods
- Generates JSON configuration files with detailed information

### 3. Lifecycle Management
- Real-time monitoring of Pod status changes (create, update, delete)
- Automatic cleanup of interface files for deleted Pods
- Ensures accurate resource usage

## Why iftop-exporter-k8s-helper?

### Problem Background
In Kubernetes clusters, directly starting `iftop` processes for all Pods is impractical:
- **Resource Consumption**: Large numbers of `iftop` processes consume significant system resources
- **Performance Impact**: May affect network performance of nodes and containers
- **Management Complexity**: Difficult to manage and maintain numerous monitoring processes

### Solution
`iftop-exporter-k8s-helper` provides the following solutions:

1. **Selective Monitoring**: Only monitors specific Pods of interest
2. **Dynamic Discovery**: Automatically discovers and configures network interfaces
3. **Resource Optimization**: Avoids unnecessary resource consumption
4. **Automated Management**: Reduces manual configuration and maintenance work

## Configuration

### Selector Syntax
```bash
# Basic format
selectorName:label1key==value1,label2key!=value2

# Examples
app:app==nginx,env==production
monitoring:monitoring==true
```

### Supported Label Operators
- `=` or `==`: Equals
- `!=`: Not equals
- Omit operator: Check if label key exists

## Integration with iftop-exporter

`iftop-exporter` monitors changes in the dynamic directory through [fsnotify](https://github.com/fsnotify/fsnotify):

1. **File Creation**: When helper creates interface files, exporter automatically starts corresponding `iftop` processes
2. **File Deletion**: When helper deletes interface files, exporter automatically stops corresponding `iftop` processes
3. **Real-time Sync**: Ensures monitoring status stays synchronized with Pod status
