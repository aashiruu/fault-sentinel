# Security Model & Kubernetes RBAC Permissions

`fault-sentinel` is an experimental chaos tool designed to run locally or inside a Kubernetes cluster under explicit service accounts.

## Required Least-Privilege RBAC
To execute pod terminations and networkExec calls, apply the following ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: fault-sentinel-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "delete"]
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["create", "get"]

## Linux Capabilities
Network injection via tc requires the target container (or fault-sentinel when executed as a sidecar) to hold the CAP_NET_ADMIN capability in its SecurityContext.
