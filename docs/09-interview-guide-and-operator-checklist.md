# Interview Guide and Repeatable Operator-Building Checklist

This chapter converts the PlatformApp Operator project into:

- A clear interview explanation
- A reusable Operator-development process
- A guide to the Go concepts used
- A troubleshooting checklist
- A production-readiness checklist
- A plan for building future Operators

The goal is not to memorize every command or line of code.

The goal is to understand the engineering pattern well enough to explain it,
reproduce it, and adapt it to a different resource.

---

## 1. What We Built

We built a Kubernetes Operator named:

```text
platformapp-operator
```

It introduces a custom Kubernetes API:

```text
API group: apps.fixnops.com
Version:   v1alpha1
Kind:      PlatformApp
Scope:     Namespaced
```

A user declares an application using:

```yaml
apiVersion: apps.fixnops.com/v1alpha1
kind: PlatformApp
metadata:
  name: nginx-demo
  namespace: platformapp-demo
spec:
  image: nginx:1.27
  replicas: 2
  port: 80
```

The Operator watches this custom resource and manages:

- A Kubernetes Deployment
- A ClusterIP Service
- Owner references
- Application status
- Availability conditions

It also detects updates and repairs deleted managed resources.

---

## 2. The Main Operator Pattern

The entire project can be remembered using this sequence:

```text
Define -> Observe -> Reconcile -> Own -> Report -> Retry -> Test -> Package -> Secure -> Operate
```

### Define

Define the custom API and CRD.

### Observe

Watch custom resources and owned child resources.

### Reconcile

Compare desired state with actual state and move the cluster toward the desired
state.

### Own

Use owner references so Kubernetes understands the relationship between the
custom resource and its child resources.

### Report

Update status and conditions to communicate observed state.

### Retry

Return errors so controller-runtime applies retry and rate-limiting behavior.

### Test

Verify creation, updates, idempotency, and failure handling.

### Package

Build the controller binary and container image.

### Secure

Use a dedicated ServiceAccount, least-privilege RBAC, a non-root image, security
contexts, and resource limits.

### Operate

Deploy multiple replicas with health probes, metrics, leader election,
anti-affinity, and a PodDisruptionBudget.

---

## 3. End-to-End Architecture

The control flow is:

```text
User
  |
  | kubectl apply
  v
Kubernetes API server
  |
  | stores PlatformApp
  v
PlatformApp controller
  |
  | reconciles desired state
  +--------------------+
  |                    |
  v                    v
Deployment          Service
  |
  v
ReplicaSet
  |
  v
Application Pods
```

Status flows back through the API server:

```text
Deployment readiness
        |
        v
PlatformApp controller
        |
        v
PlatformApp status
```

The Operator is a control-plane component for the application. It is not in the
application's network request path.

---

## 4. Thirty-Second Interview Answer

> I built a Kubernetes Operator in Go using Kubebuilder and
> controller-runtime. I defined a namespaced `PlatformApp` custom resource in
> the `apps.fixnops.com/v1alpha1` API group. Its spec accepts an image, replica
> count, and port. The reconciler watches PlatformApp resources and creates or
> updates a Deployment and ClusterIP Service using idempotent
> `CreateOrUpdate` operations. I configured owner references for garbage
> collection and child-resource watches for self-healing. The controller
> reports observed generation, ready replicas, and standard conditions. I
> added failure status, retry behavior, envtest coverage, least-privilege RBAC,
> a distroless non-root image, secure metrics, health probes, resource limits,
> two controller replicas, leader election, anti-affinity, and a
> PodDisruptionBudget.

---

## 5. Two-Minute Interview Answer

> I created the project with Kubebuilder using the Go v4 plugin and the module
> `github.com/fixnops/platformapp-operator`. I selected
> `apps.fixnops.com/v1alpha1` as the API and created a namespaced
> `PlatformApp` kind.
>
> The spec includes a required container image, an optional replica pointer
> defaulted to one and validated between one and ten, and a required port
> validated between one and 65535. Kubebuilder markers generate the CRD
> OpenAPI schema, defaults, validation rules, status subresource, generated
> DeepCopy methods, and RBAC manifests.
>
> In the reconciler, I retrieve the PlatformApp by namespaced name. A NotFound
> result is treated as successful because the resource may have been deleted.
> For an existing resource, I calculate desired values and use
> `controllerutil.CreateOrUpdate` to manage a Deployment and ClusterIP Service.
> I use consistent labels for Deployment selectors, Pod labels, and Service
> selectors. I set controller owner references so Kubernetes garbage
> collection understands the ownership relationship.
>
> The controller watches the PlatformApp and owns Deployments and Services.
> That means changes to the custom resource and relevant changes to its child
> resources enqueue reconciliation. This provides self-healing if a managed
> child resource is deleted or modified.
>
> I report `observedGeneration`, `readyReplicas`, and Available, Progressing,
> and Degraded conditions through the status subresource. When child-resource
> reconciliation fails, the controller attempts to record degraded status and
> returns the original error so controller-runtime retries with rate limiting.
>
> I tested creation, updates, idempotency, Deployment failure, and Service
> failure with envtest. I packaged the manager in a multi-stage Docker build
> using a distroless non-root runtime image and deployed it to k3d.
>
> For production-oriented operation, I used a dedicated ServiceAccount,
> least-privilege RBAC, health probes, authenticated HTTPS metrics, restricted
> security contexts, resource requests and limits, two replicas, leader
> election through a Kubernetes Lease, preferred pod anti-affinity, and a
> PodDisruptionBudget.

---

## 6. Repository Structure

The important project paths are:

```text
api/v1alpha1/
cmd/
config/
docs/
internal/controller/
test/
Dockerfile
Makefile
PROJECT
README.md
go.mod
go.sum
```

| Path | Purpose |
|---|---|
| `api/v1alpha1` | Go definitions for the custom API |
| `internal/controller` | Reconciliation logic and controller tests |
| `cmd/main.go` | Manager startup and runtime configuration |
| `config/crd` | Generated CRD manifests |
| `config/rbac` | Generated and scaffolded RBAC |
| `config/manager` | Controller manager Deployment and PDB |
| `config/default` | Default Kustomize composition |
| `config/samples` | Example PlatformApp resources |
| `test` | Test infrastructure and end-to-end scaffolding |
| `Dockerfile` | Multi-stage controller image |
| `Makefile` | Build, test, generation and deployment commands |
| `PROJECT` | Kubebuilder project metadata |
| `go.mod` | Go module and dependencies |

---

## 7. API Design Explanation

The desired state is represented by:

```go
type PlatformAppSpec struct {
    Image    string `json:"image"`
    Replicas *int32 `json:"replicas,omitempty"`
    Port     int32  `json:"port"`
}
```

The observed state is represented by:

```go
type PlatformAppStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
}
```

The key rule is:

```text
spec   = what the user wants
status = what the controller observes
```

Users and automation normally modify `spec`.

The Operator modifies `status`.

---

## 8. Why Replicas Is a Pointer

The field is:

```go
Replicas *int32
```

A pointer distinguishes:

```text
nil     = the user omitted the field
value   = the user explicitly supplied a value
```

If omitted, Kubernetes applies the CRD default of one.

The controller also defensively handles `nil`:

```go
desiredReplicas := int32(1)

if platformApp.Spec.Replicas != nil {
    desiredReplicas = *platformApp.Spec.Replicas
}
```

This avoids nil-pointer errors and makes the default behavior explicit.

---

## 9. Kubebuilder Markers

Examples include:

```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
```

```go
// +kubebuilder:default:=1
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=10
```

```go
// +kubebuilder:subresource:status
```

```go
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps,verbs=get;list;watch
```

These comments are inputs to code and manifest generators.

After changing markers or API fields, run:

```bash
make manifests
make generate
```

In this project, commands such as `make test` and `make build` also invoke
relevant generation and validation targets.

---

## 10. Reconciliation Explanation

The reconciliation function receives a request containing:

```text
namespace
name
```

It does not receive the complete object as authoritative input.

The controller retrieves the latest object from the Kubernetes API:

```go
platformApp := &appsv1alpha1.PlatformApp{}

err := r.Get(ctx, req.NamespacedName, platformApp)
```

Then it performs these logical steps:

1. Fetch the PlatformApp.
2. Ignore NotFound because deletion is a normal event.
3. Calculate desired replicas.
4. Build consistent labels.
5. Create or update the Deployment.
6. Establish its controller owner reference.
7. Create or update the Service.
8. Establish its controller owner reference.
9. Read Deployment readiness.
10. Update PlatformApp status if necessary.
11. Return success or an error.

---

## 11. Desired State Versus Actual State

Reconciliation continually compares:

```text
desired state: PlatformApp spec
actual state:  Kubernetes Deployment, Service and Pod status
```

The controller does not run only once.

Events can trigger reconciliation repeatedly.

Examples include:

- A PlatformApp is created
- Its spec is changed
- Its Deployment is changed
- Its Service is deleted
- Deployment status changes
- The controller restarts
- A retry occurs after an error

The controller must therefore be safe when invoked many times.

---

## 12. Idempotency

An idempotent reconciler can process the same desired state repeatedly without
creating unnecessary changes.

We used:

```go
controllerutil.CreateOrUpdate(...)
```

This provides three common outcomes:

```text
created
updated
unchanged
```

Our tests verified that stable repeated reconciliation does not change the
resource versions of the PlatformApp, Deployment, or Service.

Idempotency is essential for:

- Event duplication
- Controller restarts
- Error retries
- Leader failover
- Status events
- Child-resource events

---

## 13. Ownership and Garbage Collection

We used:

```go
controllerutil.SetControllerReference(
    platformApp,
    deployment,
    r.Scheme,
)
```

and the same pattern for the Service.

This creates an owner reference from each child to the PlatformApp.

Benefits include:

- Kubernetes understands the parent-child relationship
- Child events can map back to the owner
- Garbage collection can delete children after the owner is deleted
- Other controllers can identify which controller owns the object

The owner reference includes:

```text
apiVersion
kind
name
UID
controller: true
blockOwnerDeletion: true
```

---

## 14. Watching Child Resources

Controller setup includes the primary custom resource and owned children:

```go
return ctrl.NewControllerManagedBy(mgr).
    For(&appsv1alpha1.PlatformApp{}).
    Owns(&appsv1.Deployment{}).
    Owns(&corev1.Service{}).
    Named("platformapp").
    Complete(r)
```

This means reconciliation can be triggered by:

- PlatformApp events
- Owned Deployment events
- Owned Service events

This enables self-healing.

When we manually deleted the managed Deployment, the Operator received the
child-resource event and recreated it.

---

## 15. Status and Conditions

The status fields are:

```text
observedGeneration
readyReplicas
conditions
```

`observedGeneration` tells clients which spec generation the controller most
recently processed.

A useful comparison is:

```text
metadata.generation == status.observedGeneration
```

If they differ, status may not yet represent the newest desired state.

Our conditions are:

| Condition | Meaning |
|---|---|
| `Available` | Requested replicas are ready |
| `Progressing` | The application is moving toward desired readiness |
| `Degraded` | Reconciliation encountered a failure |

Each condition includes:

```text
type
status
reason
message
observedGeneration
lastTransitionTime
```

---

## 16. Error Handling and Retries

When Deployment or Service reconciliation fails, the controller:

1. Logs the failure.
2. Attempts to update status as Degraded.
3. Returns the reconciliation error.

Returning the error is important because controller-runtime can retry using a
rate-limited work queue.

The controller does not use an uncontrolled retry loop or fixed sleep inside
`Reconcile`.

If status updating also fails, the controller preserves both errors using:

```go
errors.Join(...)
```

This avoids hiding either the operational error or the status-update error.

---

## 17. Testing Strategy

We used controller-runtime envtest.

Envtest runs real Kubernetes API-server and etcd test binaries, but it does not
run:

- kubelet
- scheduler
- controller manager
- real Pods
- networking

It is well suited to testing:

- CRD validation
- Kubernetes API persistence
- Reconciler client behavior
- Created resource specifications
- Owner references
- Status updates
- Idempotency
- Failure handling

Our tests cover:

1. Deployment and Service creation
2. Initial status behavior
3. Image and replica updates
4. Observed-generation updates
5. Stable repeated reconciliation
6. Deployment reconciliation failure
7. Service reconciliation failure

---

## 18. Why Failure Tests Matter

Success-only tests prove the happy path.

Production systems also need predictable behavior when dependencies reject an
operation.

We created a Deployment with an incompatible immutable selector to test
Deployment failure.

We created a Service controlled by a ConfigMap to test owner-reference
conflict and Service failure.

The tests verified:

- An error was returned
- Reconciliation stopped at the correct stage
- Degraded became true
- The correct reason was recorded
- The original failure was not silently swallowed

---

## 19. Container Image

The Dockerfile uses a multi-stage build.

The builder stage:

- Downloads Go modules
- Copies the source
- Compiles a statically linked Linux manager binary

The runtime stage:

- Uses a small distroless image
- Runs as non-root UID and GID `65532`
- Contains the manager binary
- Uses `/manager` as the entrypoint

We built:

```text
platformapp-operator:v0.1.0-local
```

Because the k3d nodes are Linux ARM64, the image contains a Linux ARM64 binary,
not the local Darwin binary created by `make build`.

---

## 20. Local Binary Versus Container Binary

Running:

```bash
make build
```

on the Mac creates:

```text
bin/manager
```

That binary is:

```text
Darwin ARM64
```

It can run on the Mac but not inside a Linux Kubernetes node.

The Docker build creates a Linux binary inside the builder image.

This distinction is essential in cross-platform development.

---

## 21. k3d Image Import

The local image was imported with:

```bash
k3d image import \
  platformapp-operator:v0.1.0-local \
  --cluster operator-lab
```

This made the image available to the k3d node container runtimes without
pushing it to an external registry.

That approach is appropriate for local development.

Production clusters normally pull versioned images from a trusted registry.

---

## 22. In-Cluster Deployment

The deployed installation includes:

- Namespace
- CRD
- ServiceAccount
- RBAC Roles and bindings
- Metrics Service
- Controller Deployment
- PodDisruptionBudget

The Operator runs in:

```text
platformapp-operator-system
```

The Deployment is:

```text
platformapp-operator-controller-manager
```

Deployment is performed with:

```bash
make deploy IMG=platformapp-operator:v0.1.0-local
```

---

## 23. Least-Privilege RBAC

The Operator requires only the permissions used by its controller.

PlatformApps:

```text
get
list
watch
```

PlatformApp status:

```text
get
update
patch
```

Deployments:

```text
get
list
watch
create
update
```

Services:

```text
get
list
watch
create
update
```

It does not currently need explicit delete permissions because Kubernetes
garbage collection deletes owned resources when the PlatformApp is deleted.

It does not currently need finalizer permissions because the Operator does not
manage external resources.

We verified permissions using:

```bash
kubectl auth can-i ...
```

Least privilege reduces the impact of a compromised controller.

---

## 24. Runtime Security

The Operator uses a restricted runtime configuration including:

```text
runAsNonRoot: true
seccompProfile: RuntimeDefault
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities.drop: ALL
```

The image also declares:

```text
USER 65532:65532
```

Security exists at multiple layers:

- Image user
- Pod security context
- Container security context
- ServiceAccount
- RBAC
- Metrics authentication
- Resource isolation

No single field provides complete security.

---

## 25. Resource Requests and Limits

The manager container currently uses:

```text
Requests:
  CPU:    10m
  Memory: 64Mi

Limits:
  CPU:    500m
  Memory: 128Mi
```

Requests influence scheduling.

Limits constrain runtime consumption.

The resulting Pod QoS class is:

```text
Burstable
```

Production values should be adjusted using real monitoring data rather than
copied blindly.

---

## 26. Health and Metrics

The manager exposes:

```text
/healthz
/readyz
```

Kubernetes uses these for liveness and readiness probes.

The metrics server uses authenticated HTTPS on port:

```text
8443
```

An unauthenticated request returned:

```text
401 Unauthorized
```

A temporary ServiceAccount with the metrics-reader role successfully retrieved
Prometheus metrics using a bearer token.

The temporary ServiceAccount and binding were deleted after testing.

This demonstrated both observability and access control.

---

## 27. High Availability

The controller Deployment has:

```text
replicas: 2
```

Leader election ensures only one replica actively runs reconciliation workers.

The instances coordinate through a Kubernetes Lease named:

```text
01670a09.fixnops.com
```

The second Pod remains available as a standby.

If the leader stops renewing the Lease, another replica can acquire leadership.

---

## 28. Anti-Affinity and PodDisruptionBudget

Preferred pod anti-affinity uses:

```text
topologyKey: kubernetes.io/hostname
weight: 100
```

This encourages scheduling the two Operator Pods on different nodes.

The PodDisruptionBudget uses:

```text
minAvailable: 1
```

This protects one healthy replica during supported voluntary disruptions.

The PDB does not protect against all failures and does not prevent direct Pod
deletion.

---

## 29. Commands to Remember

You do not need to memorize every command.

Remember the purpose of these command groups.

### Generate and validate

```bash
make manifests
make generate
make fmt
make test
make build
```

### Install or deploy

```bash
make install
make deploy IMG=<image>
make undeploy
make uninstall
```

### Build an image

```bash
make docker-build IMG=<image>
make docker-push IMG=<image>
```

### Inspect resources

```bash
kubectl get platformapps -A
kubectl get deployments,pods,services -A
kubectl describe <resource>
kubectl get <resource> -o yaml
```

### Logs

```bash
kubectl logs \
  -n platformapp-operator-system \
  deployment/platformapp-operator-controller-manager
```

### RBAC verification

```bash
kubectl auth can-i <verb> <resource> \
  --as=<service-account-identity>
```

### Rollout

```bash
kubectl rollout status \
  deployment/platformapp-operator-controller-manager \
  -n platformapp-operator-system
```

### Lease

```bash
kubectl get lease \
  -n platformapp-operator-system
```

---

## 30. Go Concepts Used in This Project

You do not need all of Go before starting Operator development.

The most important concepts used here are:

### Packages and imports

```go
package controller
```

Imports allow the code to use Kubernetes and controller-runtime packages.

### Structs

```go
type PlatformAppSpec struct {
    Image string
}
```

Structs model API objects and controller dependencies.

### Methods

```go
func (r *PlatformAppReconciler) Reconcile(...) (...)
```

A method is a function attached to a receiver.

### Pointers

```go
*int32
*PlatformAppReconciler
```

Pointers allow optional fields, shared objects, and mutation.

### Interfaces

```go
client.Client
```

Interfaces describe behavior without requiring one concrete implementation.

This helps testing and abstraction.

### Error handling

```go
if err != nil {
    return ctrl.Result{}, err
}
```

Go handles errors explicitly as values.

### Closures

The mutation function passed to `CreateOrUpdate` is a closure:

```go
func() error {
    // set desired fields
    return nil
}
```

### Slices

```go
[]corev1.Container
[]metav1.Condition
```

Slices represent variable-length collections.

### Maps

```go
map[string]string
```

Maps are used for Kubernetes labels and selectors.

### Context

```go
context.Context
```

Context carries cancellation, deadlines, and request-scoped information.

### Multiple return values

```go
return ctrl.Result{}, err
```

Go functions can return multiple values.

---

## 31. What to Memorize

Memorize the engineering concepts:

- Spec is desired state
- Status is observed state
- Reconcile is level-based and repeatable
- NotFound can be a normal deletion event
- Child resources need stable names and labels
- Deployment selectors are immutable
- Owner references express lifecycle ownership
- Owned watches provide self-healing events
- Reconciliation must be idempotent
- Errors should normally be returned for retries
- Status should communicate success, progress, and failure
- RBAC should match actual operations
- Multiple replicas require leader election
- HA requires more than only increasing replica count

---

## 32. What to Look Up

It is normal to look up:

- Exact Kubebuilder command syntax
- Exact marker syntax
- Kubernetes API field names
- controller-runtime function signatures
- JSONPath expressions
- Kustomize syntax
- RBAC API groups and resources
- GitHub Actions syntax
- Webhook configuration
- Conversion strategies
- Cloud-provider SDK calls

Experienced engineers do not memorize every generated file.

They understand the architecture and know how to verify authoritative
documentation and generated output.

---

## 33. Repeatable Checklist for a New Operator

### Phase 1: Define the purpose

Answer:

- What is the custom resource?
- What desired state should users declare?
- What resources will the Operator manage?
- Is the resource namespaced or cluster-scoped?
- Does it manage only Kubernetes resources or also external systems?

### Phase 2: Initialize the project

```bash
kubebuilder init \
  --domain <domain> \
  --repo <go-module> \
  --project-name <project-name> \
  --plugins=go/v4
```

Never guess the domain, repository, or project name.

Verify:

```bash
cat PROJECT
cat go.mod
```

### Phase 3: Create the API

```bash
kubebuilder create api \
  --group <group> \
  --version <version> \
  --kind <kind>
```

Choose resource and controller generation when prompted.

### Phase 4: Design spec and status

Define:

- Required fields
- Optional fields
- Defaults
- Validation
- Status fields
- Conditions
- API compatibility expectations

### Phase 5: Generate artifacts

```bash
make manifests
make generate
```

Inspect:

```text
config/crd/bases/
api/<version>/zz_generated.deepcopy.go
config/rbac/role.yaml
```

### Phase 6: Implement reconciliation

Implement:

- Get the custom resource
- Handle NotFound
- Calculate desired state
- Create or update children
- Set owner references
- Report status
- Return useful errors

### Phase 7: Configure watches

Watch:

- The primary custom resource
- Owned Kubernetes resources
- Any additional resources whose changes affect desired state

### Phase 8: Add tests

Test:

- Creation
- Updating
- Default values
- Validation
- Ownership
- Status
- Idempotency
- Child deletion and drift
- Dependency failures
- Deletion behavior
- External API behavior where applicable

### Phase 9: Define RBAC

Grant only the verbs and resources used by the controller.

Regenerate and inspect:

```bash
make manifests
```

### Phase 10: Package

Build:

- Local manager binary
- Linux container image
- Immutable version tag
- Multi-architecture image if required

### Phase 11: Deploy

Deploy:

- CRD
- Namespace
- ServiceAccount
- RBAC
- Controller Deployment
- Metrics Service
- Supporting resources

### Phase 12: Secure

Verify:

- Non-root runtime
- Read-only root filesystem
- Dropped capabilities
- Seccomp
- Resource requests and limits
- Secure metrics
- Least-privilege RBAC
- NetworkPolicy if required

### Phase 13: Make highly available

Consider:

- Multiple replicas
- Leader election
- Lease RBAC
- Anti-affinity or topology spread
- PodDisruptionBudget
- Rolling-update behavior

### Phase 14: Release and operate

Add:

- CI tests
- Image scanning
- Multi-architecture publishing
- Release manifests
- Versioning
- Upgrade tests
- Monitoring
- Alerts
- Support and rollback procedures

---

## 34. Questions to Ask Before Managing External Resources

For an Operator managing EC2, databases, DNS, or another external service,
answer:

- How is the external object uniquely identified?
- How do retries avoid duplicate creation?
- Where is the external identifier stored?
- How are credentials provided?
- What happens when credentials expire?
- What happens when the Kubernetes resource is deleted?
- Is external deletion allowed?
- Is orphaning supported?
- Is deletion asynchronous?
- What status represents provisioning?
- What happens when the cloud API is temporarily unavailable?
- What API rate limits apply?
- How are secrets protected?
- How is ownership verified before deletion?

External-resource Operators normally require finalizers and stronger
idempotency design.

---

## 35. Common Interview Questions and Answers

### What is a Kubernetes Operator?

An Operator is a Kubernetes controller combined with custom APIs that automates
the lifecycle and operational knowledge of an application or infrastructure
component.

### What is a CRD?

A CustomResourceDefinition extends the Kubernetes API with a new resource type,
schema, scope, versions, validation, and optional subresources.

### What is a custom resource?

A custom resource is an instance of a CRD, such as a specific `PlatformApp`
object named `nginx-demo`.

### What does reconciliation mean?

Reconciliation compares desired state with actual state and performs operations
that move actual state toward desired state.

### Is reconciliation event-based or level-based?

Events enqueue work, but reconciliation should be level-based. It should read
the latest state and determine what is currently required rather than depending
on a particular event history.

### Why handle NotFound as success?

A queued object may have been deleted before reconciliation runs. If no
external cleanup is required, there is nothing left to reconcile.

### Why use owner references?

They represent lifecycle ownership, support garbage collection, and allow child
events to map back to the owning custom resource.

### What is the difference between spec and status?

Spec is user-declared desired state. Status is controller-reported observed
state.

### What is observedGeneration?

It is the custom-resource generation for which the reported status was
calculated.

### Why use conditions?

Conditions provide structured, machine-readable and human-readable information
about availability, progress, and degradation.

### Why return an error from Reconcile?

Returning an error allows controller-runtime to requeue the request using its
rate-limited retry behavior.

### What is idempotency?

Repeated reconciliation of the same desired state safely produces the same
result without duplicate or unnecessary side effects.

### How does the Operator self-heal a deleted Deployment?

The controller owns and watches the Deployment. Its deletion enqueues the
owning PlatformApp, and reconciliation recreates the missing child.

### Why use envtest?

Envtest provides a real API server and etcd for testing Kubernetes API
behavior without requiring a complete cluster.

### What does envtest not provide?

It does not run kubelet, scheduler, normal controller manager, real Pods, or
cluster networking.

### Why use least-privilege RBAC?

It limits the actions available to a compromised or defective controller and
documents the controller's real API requirements.

### Why use two Operator replicas?

They reduce controller downtime by providing an active leader and a warm
standby.

### Why is leader election necessary?

It prevents multiple controller replicas from actively reconciling the same
resources at the same time.

### What is a Kubernetes Lease?

It is a lightweight coordination resource used to record and renew leader
ownership.

### Does a PDB protect against node failure?

No. A PDB mainly limits voluntary evictions. It does not prevent unexpected
node or process failure.

### When is a finalizer required?

A finalizer is needed when deletion requires controller-managed cleanup that
Kubernetes garbage collection cannot perform, especially for external
resources.

### When is an admission webhook useful?

A webhook is useful when validation or defaulting requires logic that cannot be
expressed adequately using the CRD OpenAPI schema and declarative validation.

---

## 36. Troubleshooting Sequence

When an Operator is not working, investigate in this order.

### 1. Confirm the API exists

```bash
kubectl api-resources \
  --api-group=apps.fixnops.com
```

### 2. Inspect the custom resource

```bash
kubectl get platformapp <name> \
  -n <namespace> \
  -o yaml
```

### 3. Inspect Operator Pods

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -o wide
```

### 4. Inspect Operator logs

```bash
kubectl logs \
  -n platformapp-operator-system \
  deployment/platformapp-operator-controller-manager
```

### 5. Inspect events

```bash
kubectl get events \
  -n <namespace> \
  --sort-by='.lastTimestamp'
```

### 6. Inspect managed resources

```bash
kubectl get deployment,service,pods \
  -n <namespace>
```

### 7. Check ownership

```bash
kubectl get deployment <name> \
  -n <namespace> \
  -o jsonpath='{.metadata.ownerReferences}{"\n"}'
```

### 8. Check RBAC

```bash
kubectl auth can-i <verb> <resource> \
  --as=<operator-service-account> \
  -n <namespace>
```

### 9. Check status

Compare:

```text
metadata.generation
status.observedGeneration
status.conditions
```

### 10. Check leader election

```bash
kubectl get lease \
  -n platformapp-operator-system
```

---

## 37. Current Production-Readiness Assessment

The project currently includes:

- Custom API and CRD
- Declarative validation and defaulting
- Idempotent reconciliation
- Deployment and Service management
- Owner references
- Child-resource watches
- Self-healing behavior
- Status and conditions
- Failure reporting
- Retry behavior
- Controller behavior tests
- Container image
- In-cluster deployment
- Dedicated ServiceAccount
- Least-privilege RBAC
- Health probes
- Authenticated HTTPS metrics
- Restricted security context
- CPU and memory requests and limits
- Two replicas
- Leader election
- Pod anti-affinity
- PodDisruptionBudget
- Project documentation

This is a strong production-oriented foundation.

It is not yet accurate to claim that every production concern is complete.

---

## 38. Remaining Advanced Work

The next implementation stages are:

1. Publish an immutable multi-architecture image
2. Add GitHub Actions CI
3. Add image build and registry publishing
4. Generate versioned release manifests
5. Add release and upgrade verification
6. Plan API version evolution
7. Add admission webhooks where justified
8. Add a finalizer example
9. Build an external-resource example such as EC2
10. Add monitoring alerts and operational runbooks
11. Add end-to-end tests against a real cluster
12. Perform security and dependency scanning

Each feature should be implemented because it solves a defined requirement,
not merely to increase the number of files.

---

## 39. Recommended Next Practice Operators

### Operator 2: ScheduledBackup

Manage:

- A CronJob
- A ServiceAccount
- Optional backup status

This teaches:

- CronJob APIs
- Schedules
- Job ownership
- Retention
- Status aggregation

### Operator 3: StatefulDatabase

Manage:

- StatefulSet
- Headless Service
- PersistentVolumeClaim templates
- ConfigMap
- Secret references

This teaches:

- Stateful workloads
- Persistent storage
- Ordered Pods
- Upgrade considerations
- Immutable fields

### Operator 4: CloudInstance

Manage:

- An EC2 instance or a simulated external resource
- External identifiers
- Credentials
- Finalizers
- Asynchronous provisioning
- Error retries
- Deletion policies

This teaches the difference between Kubernetes-native and external-resource
Operators.

---

## 40. How the Next Operator Will Be Easier

The next Operator will reuse the same mental model:

```text
API
  -> spec and status

Controller
  -> observe and reconcile

Children
  -> create, update, own and watch

Reliability
  -> idempotency, status, errors and tests

Packaging
  -> image, manifests and deployment

Production
  -> RBAC, security, metrics and HA
```

The child resource may change from Deployment to CronJob, StatefulSet, or an
external cloud object.

The engineering structure remains similar.

You will not start from zero.

You will start with a repeatable process and use this repository as a working
reference.

---

## 41. Final Interview Summary

> The most important thing I learned is that an Operator is not just code that
> creates Kubernetes objects. It is a continuously running control loop. The
> API defines desired state, the reconciler observes actual state, and
> idempotent operations move the system toward the desired state. Ownership and
> watches provide lifecycle management and self-healing. Status communicates
> what the controller observed, while returned errors enable retries. Tests
> verify both successful and failed reconciliation. The controller itself must
> then be packaged, secured, monitored, and made highly available. I applied
> that complete pattern in the PlatformApp Operator and can reuse the same
> process for another Kubernetes-native or external-resource Operator.

---

## 42. Final Checklist

Before presenting an Operator project, confirm:

### API

- [ ] Group, version, and kind are intentional
- [ ] Scope is correct
- [ ] Required and optional fields are clear
- [ ] Defaults and validation are generated
- [ ] Status subresource is enabled

### Reconciliation

- [ ] NotFound is handled correctly
- [ ] Desired state is calculated explicitly
- [ ] Operations are idempotent
- [ ] Child objects have stable identities
- [ ] Owner references are set where appropriate
- [ ] Child watches are configured
- [ ] Errors are returned appropriately
- [ ] Status writes avoid unnecessary updates

### Testing

- [ ] Creation is tested
- [ ] Updates are tested
- [ ] Idempotency is tested
- [ ] Ownership is tested
- [ ] Status is tested
- [ ] Dependency failures are tested
- [ ] Deletion behavior is tested
- [ ] Real-cluster behavior is tested where envtest is insufficient

### Security

- [ ] Dedicated ServiceAccount is used
- [ ] RBAC is least privilege
- [ ] Container runs as non-root
- [ ] Privilege escalation is disabled
- [ ] Linux capabilities are dropped
- [ ] Root filesystem is read-only
- [ ] Seccomp is configured
- [ ] Secrets are not embedded in images or manifests

### Operations

- [ ] Health probes exist
- [ ] Metrics are secured
- [ ] Resource requests and limits exist
- [ ] Multiple replicas are considered
- [ ] Leader election is enabled for multiple replicas
- [ ] Scheduling across failure domains is considered
- [ ] A PDB is considered
- [ ] Logs and status expose useful failure information
- [ ] Upgrade and rollback procedures exist

### Delivery

- [ ] Tests run in CI
- [ ] Images use immutable tags or digests
- [ ] Images are scanned
- [ ] Release manifests are versioned
- [ ] Supported Kubernetes versions are documented
- [ ] Upgrade compatibility is tested
- [ ] Documentation and sample resources are current

---

## 43. Closing Perspective

You do not need to remember thousands of lines of generated YAML or Go code.

You need to remember:

```text
What state is declared?
What state exists?
How does reconciliation close the gap?
How is success or failure reported?
How is the behavior tested?
How is the controller secured and operated?
```

If you can answer those questions clearly, you understand the core of
Kubernetes Operator engineering.

The PlatformApp Operator is now both:

- A functioning controller project
- A reusable learning and interview reference

The next Operator will follow the same foundation with a different API and
different reconciliation responsibilities.