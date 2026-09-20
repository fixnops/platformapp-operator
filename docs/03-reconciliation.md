# Chapter 3: Reconciliation, Ownership, and Self-Healing

## Learning objectives

After this chapter, you should be able to explain:

- How the controller is registered with the manager
- What triggers reconciliation
- What `ctrl.Request` contains
- How the controller reads a PlatformApp
- Why NotFound is treated as success
- How Deployments and Services are created or updated
- How labels and selectors connect resources
- What owner references do
- How `.Owns()` enables child-resource watches
- Why reconciliation must be idempotent
- How self-healing works
- Which Go concepts appear in the controller

## 1. Controller registration

The controller is registered in `cmd/main.go`:

```go
if err := (&controller.PlatformAppReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(
        err,
        "Failed to create controller",
        "controller",
        "platformapp",
    )
    os.Exit(1)
}
```

The manager provides:

- A Kubernetes API client
- A shared informer cache
- A runtime scheme
- Logging
- Metrics
- Health checks
- Leader election
- Controller lifecycle management

The reconciler receives:

```go
Client: mgr.GetClient()
Scheme: mgr.GetScheme()
```

The client is used to read and write Kubernetes resources.

The scheme is used to understand the relationship between Go types and Kubernetes API types. It is required when creating controller owner references.

## 2. Reconciler structure

The reconciler is defined as:

```go
type PlatformAppReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}
```

`client.Client` is embedded in the struct. Embedding allows methods such as this:

```go
r.Client.Get(...)
```

to also be called as:

```go
r.Get(...)
```

The scheme is stored as a pointer because the reconciler uses the existing manager scheme rather than copying it.

## 3. Controller builder

The controller watches are configured in `SetupWithManager()`:

```go
func (r *PlatformAppReconciler) SetupWithManager(
    mgr ctrl.Manager,
) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&appsv1alpha1.PlatformApp{}).
        Owns(&appsv1.Deployment{}).
        Owns(&corev1.Service{}).
        Named("platformapp").
        Complete(r)
}
```

Each builder method has a specific purpose.

### `For()`

```go
For(&appsv1alpha1.PlatformApp{})
```

Declares PlatformApp as the primary resource.

Events involving a PlatformApp enqueue that PlatformApp's namespace and name for reconciliation.

### `Owns()` for Deployment

```go
Owns(&appsv1.Deployment{})
```

Watches Deployments controlled by PlatformApps.

When an owned Deployment changes, controller-runtime reads its controller owner reference and enqueues the owning PlatformApp.

### `Owns()` for Service

```go
Owns(&corev1.Service{})
```

Provides the same behavior for owned Services.

### `Named()`

```go
Named("platformapp")
```

Assigns a stable controller name used in logs and metrics.

### `Complete()`

```go
Complete(r)
```

connects the configured watches to the reconciler implementation.

## 4. What triggers reconciliation?

Reconciliation may be triggered by:

- PlatformApp creation
- PlatformApp spec update
- PlatformApp metadata update
- Owned Deployment creation
- Owned Deployment update
- Owned Deployment deletion
- Owned Service creation
- Owned Service update
- Owned Service deletion
- A retry after an earlier error
- Initial cache synchronization after manager startup

One user action can produce several reconciliation requests.

For example, changing replicas causes:

1. PlatformApp generation to change.
2. Controller to update the Deployment.
3. Deployment generation to change.
4. ReplicaSet to scale.
5. Deployment status to change.
6. Ready replica count to change.
7. PlatformApp status to change.

A controller must tolerate repeated and duplicate events.

## 5. Reconcile method signature

The method is:

```go
func (r *PlatformAppReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error)
```

### Context

`ctx` carries:

- Cancellation
- Deadlines
- Structured logging values
- Tracing information
- Request lifecycle information

It should be passed to Kubernetes client operations.

### Request

`req` contains:

```text
namespace
name
```

It does not contain the complete PlatformApp object.

The controller must retrieve the latest object from Kubernetes using the namespaced name.

### Result and error

The method returns:

```go
(ctrl.Result, error)
```

A successful result:

```go
return ctrl.Result{}, nil
```

means no explicit requeue is requested.

Returning an error tells controller-runtime to retry using its rate limiter.

A controller may also request a timed requeue:

```go
ctrl.Result{RequeueAfter: duration}
```

Our current controller is primarily event-driven and does not use periodic requeueing.

## 6. Request-scoped logger

The controller obtains a logger using:

```go
log := logf.FromContext(ctx)
```

controller-runtime adds useful context such as:

- Controller name
- Resource group
- Resource kind
- Namespaced name
- Reconciliation ID

Structured log fields are added like this:

```go
log.Info(
    "Reconciling PlatformApp",
    "namespace", platformApp.Namespace,
    "name", platformApp.Name,
    "image", platformApp.Spec.Image,
)
```

Structured logging is easier to search and process than building one large formatted string.

## 7. Reading the PlatformApp

The controller creates an empty Go object:

```go
platformApp := &appsv1alpha1.PlatformApp{}
```

It then retrieves the current resource:

```go
if err := r.Get(
    ctx,
    req.NamespacedName,
    platformApp,
); err != nil {
    // Error handling
}
```

After a successful `Get()`, `platformApp` contains:

- Metadata
- Spec
- Status
- Generation
- UID
- Resource version

The controller should use the object retrieved during this reconciliation rather than assuming information from the original event is still current.

## 8. Why NotFound is success

The resource may be deleted between event creation and reconciliation.

The controller checks:

```go
if apierrors.IsNotFound(err) {
    return ctrl.Result{}, nil
}
```

NotFound is not necessarily a controller failure. It usually means there is no longer any desired PlatformApp state to reconcile.

Returning success prevents unnecessary retries for an object that no longer exists.

Other read errors are returned:

```go
return ctrl.Result{}, err
```

Examples include:

- API server temporarily unavailable
- Authorization failure
- Network problem
- Cache or client failure

## 9. Determining desired replicas

The controller begins with a safe default:

```go
desiredReplicas := int32(1)
```

If replicas exists, it dereferences the pointer:

```go
if platformApp.Spec.Replicas != nil {
    desiredReplicas = *platformApp.Spec.Replicas
}
```

This provides defensive behavior even though the CRD also defaults replicas to one.

The variable is an `int32` because Kubernetes Deployment replicas use `*int32`.

## 10. Shared labels

The operator creates a map:

```go
labels := map[string]string{
    "app.kubernetes.io/name":       "platformapp",
    "app.kubernetes.io/instance":   platformApp.Name,
    "app.kubernetes.io/managed-by": "platformapp-operator",
}
```

These labels provide:

| Label | Purpose |
|---|---|
| `app.kubernetes.io/name` | Application type |
| `app.kubernetes.io/instance` | Specific PlatformApp instance |
| `app.kubernetes.io/managed-by` | Controller responsible for the resource |

The same labels are used for:

- Deployment metadata
- Deployment selector
- Pod template
- Service metadata
- Service selector

A Service sends traffic to Pods whose labels match its selector.

## 11. Deployment reconciliation

The controller first creates an object containing only identity:

```go
deployment := &appsv1.Deployment{
    ObjectMeta: metav1.ObjectMeta{
        Name:      platformApp.Name,
        Namespace: platformApp.Namespace,
    },
}
```

It then calls:

```go
controllerutil.CreateOrUpdate(...)
```

`CreateOrUpdate` follows this general process:

1. Try to retrieve the object by namespace and name.
2. If it does not exist, prepare it for creation.
3. Execute the mutation function.
4. Compare the previous and desired objects.
5. Create, update, or leave the object unchanged.
6. Return the performed operation.

Possible operation results include:

```text
created
updated
unchanged
```

## 12. The mutation function

The fourth argument to `CreateOrUpdate` is a Go closure:

```go
func() error {
    // Mutate desired fields
    return nil
}
```

A closure is a function that can access variables from its surrounding scope.

This closure accesses:

- `deployment`
- `labels`
- `desiredReplicas`
- `platformApp`
- `r.Scheme`

The function sets the desired Deployment fields every time it runs.

## 13. Deployment replicas

The desired replica count is assigned using:

```go
deployment.Spec.Replicas = &desiredReplicas
```

Deployment expects a pointer, so the controller passes the address of the local value.

When the PlatformApp replica count changes, reconciliation updates the Deployment.

## 14. Deployment selector

The selector is:

```go
deployment.Spec.Selector = &metav1.LabelSelector{
    MatchLabels: labels,
}
```

The Pod template receives the same labels:

```go
deployment.Spec.Template.ObjectMeta.Labels = labels
```

The selector and Pod labels must match. Otherwise, the Deployment cannot manage its Pods.

Deployment selectors are immutable after creation. Attempting to change an existing selector causes an API validation error.

We deliberately used this behavior when testing failure reporting.

## 15. Application container

The operator defines one application container:

```go
deployment.Spec.Template.Spec.Containers =
    []corev1.Container{
        {
            Name:  "application",
            Image: platformApp.Spec.Image,
            Ports: []corev1.ContainerPort{
                {
                    Name:          "app-port",
                    ContainerPort: platformApp.Spec.Port,
                    Protocol:      corev1.ProtocolTCP,
                },
            },
        },
    }
```

This demonstrates two Go concepts.

### Slice

A slice represents an ordered collection:

```go
[]corev1.Container
```

### Composite literal

A composite literal constructs a structured value:

```go
corev1.Container{
    Name:  "application",
    Image: platformApp.Spec.Image,
}
```

The image and port come directly from the PlatformApp spec.

## 16. Service reconciliation

The Service is also initialized using namespace and name:

```go
service := &corev1.Service{
    ObjectMeta: metav1.ObjectMeta{
        Name:      platformApp.Name,
        Namespace: platformApp.Namespace,
    },
}
```

The mutation function sets:

```go
service.Labels = labels
service.Spec.Selector = labels
service.Spec.Type = corev1.ServiceTypeClusterIP
```

A ClusterIP Service is reachable from within the Kubernetes cluster.

## 17. Service port

The Service port is configured as:

```go
service.Spec.Ports = []corev1.ServicePort{
    {
        Name:       "app-port",
        Port:       platformApp.Spec.Port,
        TargetPort: intstr.FromInt(
            int(platformApp.Spec.Port),
        ),
        Protocol: corev1.ProtocolTCP,
    },
}
```

`Port` is the port exposed by the Service.

`TargetPort` is the port used by the application container.

`intstr.IntOrString` allows Kubernetes target ports to be specified as either:

- A number
- A named port

Our operator converts the numeric PlatformApp port to the required Kubernetes type.

## 18. Controller owner references

Both mutation functions call:

```go
controllerutil.SetControllerReference(
    platformApp,
    deployment,
    r.Scheme,
)
```

or:

```go
controllerutil.SetControllerReference(
    platformApp,
    service,
    r.Scheme,
)
```

This adds an owner reference like:

```yaml
ownerReferences:
- apiVersion: apps.fixnops.com/v1alpha1
  kind: PlatformApp
  name: nginx-demo
  uid: ...
  controller: true
  blockOwnerDeletion: true
```

The UID is important. Kubernetes ownership refers to the exact owner object, not just another object with the same name.

## 19. Owner-reference benefits

Owner references provide three main benefits.

### Garbage collection

When the PlatformApp is deleted, Kubernetes garbage collection can delete its owned Deployment and Service.

The controller does not need to manually delete them.

### Event mapping

`.Owns()` uses the controller owner reference to map Deployment and Service events back to their PlatformApp.

### Ownership protection

Kubernetes permits only one controlling owner reference. This helps prevent multiple controllers from treating the same resource as their primary controlled object.

## 20. Self-healing

If the managed Deployment is deleted:

1. Deployment deletion produces an event.
2. `.Owns(&appsv1.Deployment{})` maps the event to the PlatformApp.
3. Reconciliation reads the PlatformApp.
4. `CreateOrUpdate` finds no Deployment.
5. The operator creates it again.

The same process applies to the Service.

Self-healing is a result of:

- Desired state remaining in PlatformApp
- Owned-resource watches
- Idempotent reconciliation

We tested Deployment recreation by deleting it and observing a new Deployment with a different UID.

## 21. Why CreateOrUpdate is useful

Without `CreateOrUpdate`, the controller would need separate logic:

1. Get the resource.
2. Detect NotFound.
3. Build and create the object.
4. Otherwise compare fields.
5. Update changed fields.
6. Handle conflicts.

`CreateOrUpdate` provides a standard pattern for this workflow.

It does not eliminate the need for careful design. The mutation function must:

- Set fields the operator owns
- Preserve fields it does not own
- Avoid unstable values
- Avoid unnecessary mutations
- Handle immutable fields correctly

## 22. Idempotency

An idempotent controller converges to a stable state.

Examples:

- Missing Deployment: create it
- Wrong replica count: update it
- Correct Deployment: leave it unchanged
- Missing Service: create it
- Correct Service: leave it unchanged

The controller records the operation result in logs:

```go
"operation", deploymentOperation
```

A stable resource should eventually report:

```text
operation: unchanged
```

One desired-state update may produce several reconciliation events while Kubernetes updates Deployment status. This is normal as long as the controller eventually becomes quiet.

## 23. Immutable fields

Some Kubernetes fields cannot change after resource creation.

Deployment selector is one example.

If an existing Deployment with the same name has a conflicting selector, `CreateOrUpdate` attempts an update and the API server returns an error.

The operator must not hide this failure. It records the problem in PlatformApp status and returns the original error for retry.

Strategies for immutable-field changes include:

- Rejecting incompatible desired changes
- Deleting and recreating the resource when safe
- Creating a replacement resource with a new name
- Using versioned child-resource names

The correct strategy depends on application requirements and disruption tolerance.

## 24. Status reconciliation

After Deployment and Service reconciliation succeed, the controller calls:

```go
statusChanged, err := r.updateStatus(
    ctx,
    platformApp,
    deployment,
    desiredReplicas,
)
```

This separates child-resource reconciliation from status calculation.

The status method:

- Reads Deployment readiness
- Sets observed generation
- Sets ready replicas
- Sets conditions
- Avoids unnecessary status patches

Status behavior is covered in the next chapter.

## 25. Successful completion

The controller returns:

```go
return ctrl.Result{}, nil
```

This means:

- Reconciliation succeeded
- No timed requeue is requested
- The controller waits for another event

Deployment readiness changes generate events because the controller owns and watches the Deployment, so a periodic polling loop is not necessary for this example.

## 26. RBAC markers near the controller

The controller contains:

```go
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.fixnops.com,resources=platformapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
```

These comments describe the Kubernetes API operations required by the reconciliation code.

`controller-gen` converts them into the generated ClusterRole.

RBAC is covered in detail in a later chapter.

## 27. Practical verification

Create or update the PlatformApp:

```bash
kubectl apply \
  -f config/samples/apps_v1alpha1_platformapp.yaml
```

Inspect the managed resources:

```bash
kubectl get platformapps,deployments,pods,services \
  -n platformapp-demo
```

Inspect ownership:

```bash
kubectl get deployment nginx-demo \
  -n platformapp-demo \
  -o jsonpath='{.metadata.ownerReferences}{"\n"}'
```

Inspect Service selection:

```bash
kubectl get service nginx-demo \
  -n platformapp-demo \
  -o jsonpath='{.spec.selector}{"\n"}'
```

Inspect selected endpoints:

```bash
kubectl get endpointslice \
  -n platformapp-demo \
  -l kubernetes.io/service-name=nginx-demo
```

Test application connectivity:

```bash
kubectl run platformapp-client \
  -n platformapp-demo \
  --image=busybox:1.36 \
  --restart=Never \
  --rm -i \
  -- wget -qO- http://nginx-demo:80
```

Test a desired-state update:

```bash
kubectl patch platformapp nginx-demo \
  -n platformapp-demo \
  --type=merge \
  -p '{"spec":{"replicas":2}}'
```

Verify convergence:

```bash
kubectl get platformapps,deployments,pods \
  -n platformapp-demo
```

## 28. Self-healing verification

Record the current Deployment UID:

```bash
kubectl get deployment nginx-demo \
  -n platformapp-demo \
  -o jsonpath='{.metadata.uid}{"\n"}'
```

Delete the Deployment:

```bash
kubectl delete deployment nginx-demo \
  -n platformapp-demo
```

Wait for recreation:

```bash
kubectl get deployment \
  -n platformapp-demo \
  --watch
```

Inspect the new UID:

```bash
kubectl get deployment nginx-demo \
  -n platformapp-demo \
  -o jsonpath='{.metadata.uid}{"\n"}'
```

A different UID proves the deleted Deployment was replaced with a new object.

## 29. Common mistakes

### Using event data as the source of truth

Always retrieve the current resource during reconciliation. Events may be delayed or duplicated.

### Treating NotFound as a retryable failure

If the primary resource was deleted, there is normally nothing left to reconcile.

### Using labels that do not match selectors

Deployment selectors, Pod labels, and Service selectors must be consistent.

### Forgetting owner references

Without ownership, garbage collection and `.Owns()` event mapping do not work as intended.

### Writing create-only logic

A production controller must handle existing, changed, deleted, and already-correct resources.

### Updating every object during every reconciliation

Unnecessary updates create more events, API traffic, and resource-version changes.

### Assuming one reconcile call per user change

Kubernetes is event-driven and eventually consistent. Several reconciliations may occur while child-resource status converges.

## 30. Interview questions and answers

### What information does `ctrl.Request` contain?

It contains the namespace and name of the primary resource to reconcile, not the complete resource object.

### Why does the controller call `Get()`?

It retrieves the latest PlatformApp state from the controller-runtime cache or API client using the request's namespaced name.

### Why return success for NotFound?

The object may have been deleted after the event was queued. There is no remaining desired state to reconcile.

### What does `CreateOrUpdate` do?

It retrieves an object, runs a mutation function, and creates, updates, or leaves the resource unchanged based on the resulting desired state.

### Why use owner references?

They enable Kubernetes garbage collection, establish controlling ownership, and allow child-resource events to map back to the PlatformApp.

### What does `.Owns()` do?

It watches owned child resources and enqueues their controlling PlatformApp when they change.

### How does self-healing work?

The PlatformApp remains the source of desired state. When an owned Deployment or Service is deleted, its event triggers reconciliation and the operator recreates it.

### Why must reconciliation be idempotent?

The same request may be processed multiple times. Idempotency ensures repeated execution converges safely without creating duplicate resources or continuous updates.

### Why use an event-driven controller instead of polling?

Watches allow the controller to respond quickly to API changes while avoiding constant polling. Timed requeues are still useful for external systems or time-based checks.

### What happens if the Service succeeds but status fails?

The controller returns the status error. controller-runtime retries, and idempotent child-resource reconciliation sees that the Deployment and Service already exist before retrying the status update.

## 31. Chapter summary

The controller turns PlatformApp desired state into concrete Kubernetes resources.

The core flow is:

```text
PlatformApp event
        ↓
Read current PlatformApp
        ↓
Create or update Deployment
        ↓
Create or update Service
        ↓
Update PlatformApp status
        ↓
Wait for another event or retry an error
```

Owner references, `.Owns()` watches, and idempotent create-or-update logic provide lifecycle management and self-healing.

The next chapter explains status conditions, failure reporting, error preservation, and controller-runtime retries.