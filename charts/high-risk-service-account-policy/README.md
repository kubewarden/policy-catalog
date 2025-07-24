[![Stable](https://img.shields.io/badge/status-stable-brightgreen?style=for-the-badge)](https://github.com/kubewarden/community/blob/main/REPOSITORIES.md#stable)

# High-Risk Service Account Blocker

The policy can be configured to define a list of resources and operations that
are considered privileged (for example, `LIST, GET` Secret resources).

When a workload is defined (a Pod, Deployment, CronJob,...) the policy will use
the [Kubernetes Authorization
API](https://kubernetes.io/docs/reference/access-authn-authz/authorization/#checking-api-access)
to assess whether the ServiceAccount used by the workload has the rights to
perform any of the sensitive operations defined by the user.

Workloads that use a high-privileged ServiceAccount are rejected.

Every time a resource that uses a service account is submitted to the cluster,
the policy will query the Kubernetes authorization API to check if the given
ServiceAccount has some permissions that it shouldn't. To perform this
verification, the policy will create an
[SubjectAccessReview](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/subject-access-review-v1/)
and apply it to check the service account permissions. If the result returned
that the service account can perform such operation, the request is rejected.

When the policy builds the `SubjectAccessReview` the user set in in the
resource is defined by `system:serviceaccount:<namespace>:<service-account>`.
Where the `namespace` is the namespace from the request and the
`service-account` is the service account from the resource being deployed or
the `default` one.

The policy rejects a workload as soon as it found a blocked operation.
Therefore, it avoids hitting the Kubernetes API too many times before rejecting
the request.

## Example

The policy settings consist of a list of rules that service accounts are
prohibited from having defined in any of their associated roles and cluster
roles. The rules follow the same syntax as those defined in the
[roles](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/subject-access-review-v1/)
specification.

```yaml
blockRules:
  # For listing secrets
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["list"]

  # For executing commands in containers
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["create"]

  # Full access to all workload resources
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["*"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["*"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["*"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["*"]

  # Full access to RBAC resources within a namespace
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings"]
    verbs: ["*"]
    namespace: "mynamespace"
```

The verbs field allow the following values: `create`, `update`, `delete`,
`get`, `list`, `watch`, `proxy`, `*`, `patch` and `deletecollection`.

For example, if the policy is deployed with the previous configuration in a
cluster that has a service account like this:

```yaml
---
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: super-admin-sa
  namespace: default

---
# Powerful Role with all requested permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: super-admin-role
  namespace: default
rules:
  # For listing secrets
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["list"]

  # For executing commands in containers
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["create"]

  # Full access to all workload resources
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["*"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["*"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["*"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["*"]
  # Full access to RBAC resources within namespace
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings"]
    verbs: ["*"]

---
# RoleBinding for namespace-scoped permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: super-admin-rolebinding
  namespace: default
subjects:
  - kind: ServiceAccount
    name: super-admin-sa
    namespace: default
roleRef:
  kind: Role
  name: super-admin-role
  apiGroup: rbac.authorization.k8s.io
```

The following deployment would not be allowed in the cluster:

```yaml
---
# Deployment using the powerful ServiceAccount
apiVersion: apps/v1
kind: Deployment
metadata:
  name: super-admin-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: super-admin-app
  template:
    metadata:
      labels:
        app: super-admin-app
    spec:
      serviceAccountName: super-admin-sa
      containers:
        - name: main
          image: nginx:alpine
          ports:
            - containerPort: 80
        - name: kubectl
          image: bitnami/kubectl:latest
          command: ["sleep", "infinity"]
```

In the previous example, the user set in the `SubjectAccessReview` resource
would be `system:serviceaccount:default:super-admin-sa`.
