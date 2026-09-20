# Chapter 7: RBAC, Runtime Security, Health, and Metrics

## Learning objectives

After this chapter, you should be able to explain:

- How an operator Pod authenticates to Kubernetes
- How ServiceAccounts, ClusterRoles, and bindings work together
- How Kubebuilder RBAC markers generate permissions
- What least privilege means
- How the operator permissions were reduced
- How Pod and container security contexts protect the manager
- How requests and limits affect scheduling and runtime
- The difference between liveness and readiness
- How secure metrics authentication and authorization work
- How metrics access was tested

## 1. Authentication versus authorization

Authentication answers:

> Who is making this request?

Authorization answers:

> Is that identity allowed to perform this action?

The operator authenticates using its Kubernetes ServiceAccount.

RBAC authorizes the operations that ServiceAccount can perform.

Both are required.

## 2. ServiceAccount identity

The operator Deployment uses:

```yaml
serviceAccountName:
  platformapp-operator-controller-manager
```

The ServiceAccount exists in:

```text
platformapp-operator-system
```

Its complete Kubernetes identity is:

```text
system:serviceaccount:platformapp-operator-system:platformapp-operator-controller-manager
```

This identity is used during authorization decisions.

## 3. Projected ServiceAccount token

Kubernetes mounts a projected volume into the Pod containing:

- Short-lived ServiceAccount token
- Cluster CA certificate
- Namespace information

The manager uses controller-runtime's in-cluster configuration to communicate with the API server.

No developer kubeconfig is copied into the image.

## 4. RBAC objects

The main RBAC relationship is:

```text
Operator Pod
    ↓
ServiceAccount
    ↓
ClusterRoleBinding
    ↓
ClusterRole
    ↓
Allowed Kubernetes API operations
```

### ServiceAccount

Provides the Pod identity.

### ClusterRole

Contains rules describing API groups, resources, and verbs.

### ClusterRoleBinding

Connects the ServiceAccount to the ClusterRole.

A Role and RoleBinding are namespace-scoped.

A ClusterRole and ClusterRoleBinding can authorize access across namespaces.

## 5. Why the manager uses a ClusterRole

PlatformApp is namespaced, but the operator watches PlatformApps in multiple namespaces.

It also manages Deployments and Services in the namespaces containing those PlatformApps.

Therefore, its main permissions are represented by a ClusterRole.

The operator itself still runs in one system namespace.

## 6. Kubebuilder RBAC markers

The controller source contains:

```go
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
```

`controller-gen` converts these comments into:

```text
config/rbac/role.yaml
```

After changing markers, regenerate RBAC:

```bash
make manifests
```

Do not normally edit the generated main role directly.

## 7. API groups in RBAC

The API groups are:

| Resource | API group |
|---|---|
| PlatformApp | `apps.fixnops.com` |
| Deployment | `apps` |
| Service | Empty core API group |

The core API group is represented by:

```go
groups=""
```

This is different from the Deployment group:

```go
groups=apps
```

## 8. Kubernetes verbs

Common RBAC verbs include:

| Verb | Purpose |
|---|---|
| `get` | Read one object |
| `list` | Read a collection |
| `watch` | Receive change events |
| `create` | Create an object |
| `update` | Replace/update an existing object |
| `patch` | Apply a partial change |
| `delete` | Delete an object |

Permissions should match the actual client operations used by the controller.

## 9. Why list and watch are necessary

The Reconcile method mainly calls `Get()`.

However, controller-runtime uses an informer cache.

The cache:

1. Lists existing objects.
2. Watches for changes.
3. Stores current objects locally.
4. Generates reconciliation events.

Therefore, watched resources require:

```text
get
list
watch
```

## 10. Original broad permissions

The initial generated markers allowed operations such as:

- Create and delete PlatformApps
- Patch and delete Deployments
- Patch and delete Services
- Update PlatformApp finalizers

The controller did not use these operations.

Keeping unused permissions would violate least privilege.

## 11. Least-privilege permissions

The final PlatformApp permission is:

```text
get, list, watch
```

The controller reads and watches PlatformApps but does not create or delete them.

Status has separate permission:

```text
get, update, patch
```

Deployments and Services use:

```text
get, list, watch, create, update
```

The controller uses `CreateOrUpdate`, but does not explicitly delete these resources.

Kubernetes garbage collection handles owned-resource deletion.

## 12. Why finalizer permission was removed

The generated controller initially had:

```text
platformapps/finalizers: update
```

The current operator does not implement a finalizer.

Granting finalizer permission before the controller uses it would be unnecessary.

The permission will be restored only when finalizer behavior is implemented.

## 13. Verifying RBAC

The operator identity was stored as:

```bash
OPERATOR_SA='system:serviceaccount:platformapp-operator-system:platformapp-operator-controller-manager'
```

Expected allowed operations returned `yes`:

```bash
kubectl auth can-i get \
  platformapps.apps.fixnops.com \
  --as="$OPERATOR_SA" \
  --all-namespaces
```

```bash
kubectl auth can-i update \
  deployments.apps \
  --as="$OPERATOR_SA" \
  --all-namespaces
```

```bash
kubectl auth can-i create \
  services \
  --as="$OPERATOR_SA" \
  --all-namespaces
```

Expected denied operations returned `no`:

```bash
kubectl auth can-i delete \
  deployments.apps \
  --as="$OPERATOR_SA" \
  --all-namespaces
```

```bash
kubectl auth can-i create \
  platformapps.apps.fixnops.com \
  --as="$OPERATOR_SA" \
  --all-namespaces
```

```bash
kubectl auth can-i update \
  platformapps.apps.fixnops.com/finalizers \
  --as="$OPERATOR_SA" \
  --all-namespaces
```

This proved least privilege through live authorization checks.

## 14. Leader-election permissions

Leader election uses a Lease in:

```text
platformapp-operator-system
```

The operator has a namespaced Role and RoleBinding for leader-election resources.

These permissions are separate from the main application-management ClusterRole.

This limits Lease operations to the operator namespace.

## 15. Metrics authentication permissions

The secure metrics server delegates authentication and authorization to Kubernetes.

The manager needs permission to create:

- TokenReview
- SubjectAccessReview

The generated metrics-auth ClusterRole and binding provide these permissions to the operator ServiceAccount.

These permissions allow the metrics server to ask Kubernetes:

- Is this bearer token valid?
- Is this identity allowed to read `/metrics`?

They do not automatically allow every identity to read metrics.

## 16. Metrics reader role

The generated metrics-reader ClusterRole grants access to the non-resource URL:

```text
/metrics
```

A monitoring ServiceAccount must be bound to that role.

This separates:

- The identity serving metrics
- The identity reading metrics

Production monitoring systems should use a dedicated ServiceAccount.

## 17. Pod security context

The Pod security context includes:

```yaml
securityContext:
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault
```

### `runAsNonRoot`

Kubernetes rejects container startup if the runtime determines it would run as root.

The image declares UID/GID:

```text
65532:65532
```

### `RuntimeDefault` seccomp

The container uses the runtime's default system-call filtering profile.

This reduces exposure to unnecessary Linux system calls.

## 18. Container security context

The manager container uses:

```yaml
securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
```

### Read-only root filesystem

The container cannot write to its root filesystem.

The operator currently does not require local persistent writes.

### No privilege escalation

The process cannot gain additional privileges through mechanisms such as setuid binaries.

### Drop all capabilities

Linux capabilities divide root privileges into individual units.

The manager requires none, so all are dropped.

## 19. Image-level user and Kubernetes enforcement

The Dockerfile declares:

```dockerfile
USER 65532:65532
```

Kubernetes also declares:

```yaml
runAsNonRoot: true
```

These controls complement each other.

The image selects a non-root user.

Kubernetes enforces that the workload cannot run as root.

## 20. Live security verification

The running Pod reported:

```text
pod.runAsNonRoot=true
pod.seccompProfile=RuntimeDefault
container.allowPrivilegeEscalation=false
container.readOnlyRootFilesystem=true
container.dropCapabilities=["ALL"]
```

The image reported:

```text
image-user=65532:65532
```

This confirmed that both image-level and Kubernetes-level controls were active.

## 21. Resource requests and limits

The manager container declares:

```yaml
resources:
  requests:
    cpu: 10m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 128Mi
```

### CPU request

```text
10m
```

reserves 0.01 CPU for scheduling purposes.

### CPU limit

```text
500m
```

allows up to half a CPU before throttling.

### Memory request

```text
64Mi
```

is used by the scheduler when placing the Pod.

### Memory limit

```text
128Mi
```

is the maximum container memory before an out-of-memory termination may occur.

## 22. QoS class

The Pod reported:

```text
Burstable
```

It is Burstable because requests and limits exist but are not equal.

A Guaranteed Pod requires each container's CPU and memory requests to equal its limits.

Burstable is reasonable for this small operator because it reserves a baseline while allowing controlled bursts.

Production values should be based on observed usage rather than copied blindly.

## 23. Health endpoints

The manager registers:

```go
mgr.AddHealthzCheck(
    "healthz",
    healthz.Ping,
)
```

and:

```go
mgr.AddReadyzCheck(
    "readyz",
    healthz.Ping,
)
```

The endpoints are served on:

```text
8081
```

Paths:

```text
/healthz
/readyz
```

## 24. Liveness versus readiness

### Liveness

Answers:

> Is the manager process healthy?

Failure causes kubelet to restart the container after the configured threshold.

### Readiness

Answers:

> Is the manager ready to serve?

Failure marks the Pod NotReady.

The container can remain running while not ready.

For an operator without a normal traffic-serving Service, readiness still provides useful rollout and operational state.

## 25. Probe configuration

The manager Deployment includes:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 15
  periodSeconds: 20
```

and:

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 10
```

The running Pod reported:

```text
Ready=True
Restart Count=0
```

## 26. Direct health verification

The health port was forwarded:

```bash
kubectl port-forward \
  -n platformapp-operator-system \
  deployment/platformapp-operator-controller-manager \
  8081:8081
```

The endpoint returned:

```bash
curl -i http://localhost:8081/healthz
```

```text
HTTP/1.1 200 OK
ok
```

Readiness can be checked using:

```bash
curl -i http://localhost:8081/readyz
```

## 27. Metrics configuration

The manager serves metrics on:

```text
8443
```

The endpoint uses HTTPS and Kubernetes authentication and authorization.

HTTP/2 is disabled by default as a security hardening measure.

The manager logs confirmed:

```text
Starting metrics server
Serving metrics server
secure=true
```

## 28. Unauthenticated metrics test

The metrics port was forwarded:

```bash
kubectl port-forward \
  -n platformapp-operator-system \
  deployment/platformapp-operator-controller-manager \
  8443:8443
```

An unauthenticated request:

```bash
curl -ki https://localhost:8443/metrics
```

returned:

```text
HTTP/1.1 401 Unauthorized
```

This proved that metrics were not publicly readable.

## 29. Authenticated metrics test

A temporary ServiceAccount was created:

```bash
kubectl create serviceaccount \
  platformapp-metrics-test \
  -n platformapp-operator-system
```

It was bound to the metrics-reader role:

```bash
kubectl create clusterrolebinding \
  platformapp-metrics-test \
  --clusterrole=platformapp-operator-metrics-reader \
  --serviceaccount=platformapp-operator-system:platformapp-metrics-test
```

A short-lived token was generated:

```bash
METRICS_TOKEN="$(
  kubectl create token \
    platformapp-metrics-test \
    -n platformapp-operator-system \
    --duration=10m
)"
```

The authenticated request succeeded:

```bash
curl -ksS \
  -H "Authorization: Bearer ${METRICS_TOKEN}" \
  https://localhost:8443/metrics
```

## 30. Metrics observed

The endpoint exposed metrics such as:

```text
controller_runtime_active_workers
controller_runtime_max_concurrent_reconciles
controller_runtime_reconcile_errors_total
controller_runtime_reconcile_panics_total
controller_runtime_reconcile_time_seconds
```

The controller reported:

```text
controller_runtime_reconcile_errors_total{controller="platformapp"} 0
```

and:

```text
controller_runtime_max_concurrent_reconciles{controller="platformapp"} 1
```

## 31. Why metrics matter

Metrics help detect:

- Reconciliation errors
- Slow reconciliation
- Workqueue growth
- Worker saturation
- Controller panics
- Process CPU and memory changes
- Go runtime problems
- Certificate watcher errors

A monitoring system can scrape and alert on these signals.

## 32. Temporary metrics cleanup

After testing, the temporary access was removed:

```bash
kubectl delete clusterrolebinding \
  platformapp-metrics-test
```

```bash
kubectl delete serviceaccount \
  platformapp-metrics-test \
  -n platformapp-operator-system
```

The token variable and temporary output were also removed.

Temporary test privileges should not remain in the cluster.

## 33. NetworkPolicy status

The generated project includes optional NetworkPolicy scaffolding, but it is not currently enabled.

A production environment may restrict:

- Metrics ingress
- Webhook ingress
- API-server egress
- DNS egress
- External-provider egress

NetworkPolicy behavior depends on the cluster network plugin.

This remains a possible future hardening task.

## 34. Security is layered

No single control makes the operator secure.

The project uses multiple layers:

```text
Minimal image
    ↓
Non-root user
    ↓
Restricted security context
    ↓
Read-only filesystem
    ↓
Dropped capabilities
    ↓
Resource limits
    ↓
Dedicated ServiceAccount
    ↓
Least-privilege RBAC
    ↓
Authenticated metrics
    ↓
TLS
```

If one layer fails, other controls still reduce risk.

## 35. Common mistakes

### Giving the operator cluster-admin

This hides missing RBAC rules and creates excessive risk.

### Granting every CRUD verb

Permissions should correspond to actual controller operations.

### Forgetting list and watch

Informer caches require them for watched resources.

### Granting finalizer access before implementation

Permissions should be introduced with the feature that uses them.

### Running as root

Most Go-based controllers do not require root.

### Making the root filesystem writable unnecessarily

Controllers should use explicit writable volumes only when required.

### Exposing unauthenticated metrics

Metrics may reveal cluster names, resource names, failure rates, and operational behavior.

### Treating health and readiness as identical concepts

They cause different kubelet behavior.

### Copying resource limits without measuring

Limits that are too low cause instability. Limits that are too high reduce scheduling efficiency and protection.

## 36. Interview questions and answers

### How does the operator authenticate?

The Pod uses its Kubernetes ServiceAccount and projected token through controller-runtime's in-cluster configuration.

### How does RBAC authorization work?

A ClusterRoleBinding connects the operator ServiceAccount to a ClusterRole containing resource and verb rules.

### Why does a namespaced CR use a ClusterRole?

The operator watches PlatformApps and manages resources across multiple namespaces.

### How did you implement least privilege?

I mapped the controller's actual client operations to RBAC verbs, regenerated the ClusterRole, and verified allowed and denied actions using `kubectl auth can-i` with ServiceAccount impersonation.

### Why does the operator need list and watch?

controller-runtime's informer cache lists existing resources and watches changes to generate reconciliation events.

### Why remove delete permission for Deployments and Services?

The controller does not call Delete. Kubernetes garbage collection deletes owned resources when the PlatformApp is deleted.

### What is the difference between liveness and readiness?

Liveness determines whether kubelet should restart the container. Readiness determines whether the Pod is considered ready to serve.

### How are metrics protected?

The metrics server uses HTTPS and Kubernetes delegated authentication and authorization. Unauthenticated requests return 401.

### Why use a distroless non-root container plus Kubernetes securityContext?

The image minimizes available tools and selects a non-root user, while Kubernetes enforces non-root execution, seccomp, read-only filesystem, no privilege escalation, and dropped capabilities.

### What does Burstable QoS mean?

The Pod has requests and limits, but they are not equal. It receives a scheduling reservation while being allowed controlled resource bursts.

## 37. Reusable security checklist

For another operator:

1. Create a dedicated ServiceAccount.
2. Map each API call to required RBAC.
3. Generate and review the ClusterRole.
4. Test allowed operations.
5. Test denied operations.
6. Avoid cluster-admin.
7. Run as a numeric non-root user.
8. Enable RuntimeDefault seccomp.
9. Disable privilege escalation.
10. Drop all capabilities.
11. Use a read-only root filesystem.
12. Configure measured requests and limits.
13. Add liveness and readiness probes.
14. Serve metrics securely.
15. Use a dedicated metrics reader.
16. Remove temporary test identities.
17. Consider NetworkPolicy.

## 38. Chapter summary

The operator runs with a dedicated identity and only the permissions required by its reconciliation logic.

Its runtime is protected by:

- Non-root execution
- Distroless image
- Seccomp
- Read-only filesystem
- No privilege escalation
- No Linux capabilities
- Resource controls

Its health is visible through probes, and its behavior is observable through authenticated HTTPS metrics.

The next chapter explains multiple replicas, Lease-based leader election, Pod anti-affinity, PodDisruptionBudget, rolling updates, and tested leader failover.