# Chapter 4: Status, Conditions, Errors, and Retries

## Learning objectives

After this chapter, you should be able to explain:

- Why Kubernetes APIs separate spec and status
- How `observedGeneration` detects stale status
- How ready replicas are calculated
- How Kubernetes conditions work
- Why the controller uses `DeepCopy`
- Why unchanged status should not be patched
- How merge patches reduce update conflicts
- How reconciliation failures are reported
- Why the original error is returned
- How controller-runtime retries errors
- Why `errors.Join` preserves multiple failures

## 1. Desired state versus observed state

PlatformApp has two main sections:

```yaml
spec:
  image: nginx:1.27
  replicas: 2
  port: 80
```

and:

```yaml
status:
  observedGeneration: 4
  readyReplicas: 2
  conditions: []
```

`spec` is the desired state supplied by the user.

`status` is the observed state reported by the controller.

This separation is fundamental to Kubernetes API design.

The user says what should happen. The controller reports what has happened.

## 2. Why status matters

Without status, users would need to inspect:

- Deployment replicas
- Deployment conditions
- Pods
- ReplicaSets
- Services
- Controller logs

PlatformApp status summarizes the most important application state directly on the custom resource.

Useful commands include:

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o yaml
```

and:

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o jsonpath='{.status}'
```

Status is also usable by:

- Monitoring systems
- GitOps health checks
- Automation
- Other controllers
- User interfaces
- Support tooling

## 3. Status fields

The API defines:

```go
type PlatformAppStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
}
```

### ObservedGeneration

Records the PlatformApp generation processed by the controller.

### ReadyReplicas

Reports the Deployment's current ready replica count.

### Conditions

Reports structured state, reason, message, generation, and transition time.

## 4. Generation

Kubernetes maintains:

```text
metadata.generation
```

When a user changes the PlatformApp spec, Kubernetes increments generation.

For example:

```text
Before patch: generation=3
After patch:  generation=4
```

Status-only updates do not increment generation.

This lets clients distinguish desired-state changes from status changes.

## 5. Observed generation

The controller records:

```go
platformApp.Status.ObservedGeneration =
    platformApp.Generation
```

Comparing the values answers an important question:

> Does the current status describe the latest desired state?

Example:

```text
generation=4
observedGeneration=4
```

The controller has processed generation 4.

Example:

```text
generation=5
observedGeneration=4
```

The user submitted generation 5, but status still reflects generation 4.

This may be temporary while reconciliation is queued or running.

## 6. Ready replicas

The controller reads:

```go
readyReplicas :=
    deployment.Status.ReadyReplicas
```

This value comes from the Kubernetes Deployment controller.

It does not simply count desired replicas. It represents replicas considered ready according to Deployment and Pod readiness.

The controller writes:

```go
platformApp.Status.ReadyReplicas =
    readyReplicas
```

## 7. Determining availability

The controller calculates:

```go
isAvailable :=
    readyReplicas >= desiredReplicas
```

If all requested replicas are ready:

```text
Available=True
Progressing=False
Degraded=False
```

If fewer replicas are ready:

```text
Available=False
Progressing=True
Degraded=False
```

If reconciliation fails:

```text
Available=False
Progressing=False
Degraded=True
```

These conditions describe different dimensions of state.

## 8. Kubernetes conditions

The controller uses:

```go
metav1.Condition
```

A condition contains:

| Field | Purpose |
|---|---|
| `type` | Aspect of state being described |
| `status` | `True`, `False`, or `Unknown` |
| `reason` | Machine-readable CamelCase reason |
| `message` | Human-readable explanation |
| `observedGeneration` | Generation associated with the condition |
| `lastTransitionTime` | Time condition status last changed |

Conditions should help both humans and automation understand the resource.

## 9. Available condition

When all replicas are ready:

```go
metav1.Condition{
    Type:               conditionAvailable,
    Status:             metav1.ConditionTrue,
    Reason:             "DeploymentAvailable",
    Message:            fmt.Sprintf(
        "%d of %d requested replicas are ready",
        readyReplicas,
        desiredReplicas,
    ),
    ObservedGeneration: platformApp.Generation,
}
```

When replicas are still becoming ready:

```go
metav1.Condition{
    Type:               conditionAvailable,
    Status:             metav1.ConditionFalse,
    Reason:             "ReplicasNotReady",
    Message:            fmt.Sprintf(
        "%d of %d requested replicas are ready",
        readyReplicas,
        desiredReplicas,
    ),
    ObservedGeneration: platformApp.Generation,
}
```

## 10. Progressing condition

When waiting for replicas:

```text
Progressing=True
Reason=WaitingForReplicas
```

When the rollout is complete:

```text
Progressing=False
Reason=DeploymentComplete
```

`Progressing=False` does not mean failure. It means the resource is no longer actively moving toward readiness because the requested state is complete.

## 11. Degraded condition

After successful Deployment and Service reconciliation:

```text
Degraded=False
Reason=ReconciliationSucceeded
```

When a child resource cannot be reconciled:

```text
Degraded=True
```

The reason identifies the failed component, for example:

```text
DeploymentReconciliationFailed
ServiceReconciliationFailed
```

The message contains the original error text.

## 12. Setting conditions safely

The controller uses:

```go
apimeta.SetStatusCondition(
    &platformApp.Status.Conditions,
    condition,
)
```

This helper:

- Adds the condition if it does not exist
- Updates the existing condition with the same type
- Preserves list uniqueness by condition type
- Maintains transition-time behavior

The API markers also declare conditions as a map-style list keyed by type.

## 13. Why status updates can trigger reconciliation

A status patch changes the PlatformApp object.

Because the controller watches PlatformApps, that change may create another reconcile event.

If the controller patches identical status every time:

1. Reconcile patches status.
2. Status update generates an event.
3. Event triggers reconcile.
4. Reconcile patches identical status again.
5. Loop continues.

The controller avoids this by checking whether status actually changed.

## 14. Taking a deep copy

Before modifying status:

```go
statusBeforeChange :=
    platformApp.DeepCopy()
```

A deep copy creates an independent object.

This is important because PlatformApp contains:

- Pointers
- Slices
- Nested structures

If both variables shared the same underlying condition slice, changing one could also change the comparison baseline.

## 15. Comparing status

After calculating desired status:

```go
if reflect.DeepEqual(
    statusBeforeChange.Status,
    platformApp.Status,
) {
    return false, nil
}
```

If the old and new statuses are equal:

- No API patch is sent
- No new status event is generated
- API traffic is reduced
- Stable reconciliation remains quiet

The function returns `false` to indicate that no status write was required.

## 16. Status patch

When status changed, the controller uses:

```go
r.Status().Patch(
    ctx,
    platformApp,
    client.MergeFrom(statusBeforeChange),
)
```

`r.Status()` selects the status subresource client.

`client.MergeFrom(statusBeforeChange)` creates a merge patch describing the difference between the original object and modified object.

This is preferable to replacing the entire resource because it:

- Updates only changed status fields
- Reduces the chance of overwriting unrelated changes
- Uses the dedicated status subresource
- Works with separate status RBAC

## 17. Returning whether status changed

`updateStatus()` returns:

```go
(bool, error)
```

Possible results:

| Return | Meaning |
|---|---|
| `false, nil` | Status already matched |
| `true, nil` | Status was patched |
| `false, error` | Status patch failed |

The reconciler logs only when a patch occurred:

```go
if statusChanged {
    log.Info(
        "Updated PlatformApp status",
        ...
    )
}
```

This keeps logs meaningful.

## 18. Successful status example

A successfully reconciled PlatformApp produced:

```yaml
status:
  observedGeneration: 3
  readyReplicas: 3
  conditions:
  - type: Available
    status: "True"
    reason: DeploymentAvailable
    message: 3 of 3 requested replicas are ready
  - type: Progressing
    status: "False"
    reason: DeploymentComplete
    message: All requested replicas are ready
  - type: Degraded
    status: "False"
    reason: ReconciliationSucceeded
    message: Deployment and Service reconciliation succeeded
```

The important consistency check is:

```text
metadata.generation ==
status.observedGeneration
```

## 19. Failure handling flow

If Deployment reconciliation fails:

```go
return ctrl.Result{},
    r.handleReconcileError(
        ctx,
        platformApp,
        "DeploymentReconciliationFailed",
        err,
    )
```

If Service reconciliation fails:

```go
return ctrl.Result{},
    r.handleReconcileError(
        ctx,
        platformApp,
        "ServiceReconciliationFailed",
        err,
    )
```

The helper:

1. Copies the original resource.
2. Records observed generation.
3. Sets failure conditions.
4. Avoids an unchanged patch.
5. Patches status when necessary.
6. Returns the original reconciliation error.

## 20. Failure conditions

On reconciliation failure:

```text
Available=False
Reason=ReconciliationFailed
```

```text
Progressing=False
Reason=ReconciliationFailed
```

```text
Degraded=True
Reason=<component-specific reason>
```

The Degraded message contains:

```go
reconcileErr.Error()
```

This exposes the Kubernetes API error to users inspecting the PlatformApp.

Sensitive external-system errors may require message sanitization in future operators.

## 21. Why return the original error?

The helper returns:

```go
return reconcileErr
```

This is deliberate.

If the controller updated Degraded status and then returned success, controller-runtime could treat reconciliation as complete.

Returning the original error tells controller-runtime:

- Reconciliation failed
- Record the failure metric
- Requeue the resource
- Apply rate-limited retry behavior

Status reporting and retry signaling are separate responsibilities. The controller does both.

## 22. Rate-limited retries

When `Reconcile()` returns an error, controller-runtime schedules another attempt through its workqueue rate limiter.

The delay generally increases for repeated failures, preventing a broken resource from generating a tight retry loop.

A later successful reconciliation clears the error path and normal event processing resumes.

The controller does not need to implement its own sleep loop.

Blocking or sleeping inside `Reconcile()` would waste a worker and reduce controller throughput.

## 23. Status failure while reporting another failure

A difficult case can occur:

1. Deployment reconciliation fails.
2. The controller attempts to set `Degraded=True`.
3. The status patch also fails.

There are now two important errors:

- Original reconciliation error
- Status update error

Returning only the status error would hide the original failure.

Returning only the reconciliation error would hide the status-write failure.

## 24. Joining errors

The controller uses:

```go
errors.Join(
    reconcileErr,
    fmt.Errorf(
        "patch degraded PlatformApp status: %w",
        statusErr,
    ),
)
```

`errors.Join` produces one error containing both underlying errors.

The `%w` verb wraps the status error so Go error inspection can retain the error chain.

This preserves diagnostic information for logs and retry behavior.

## 25. Go error handling concepts

Go treats errors as values.

A function declares an error return:

```go
func (...) error
```

Success is represented by:

```go
nil
```

Failure is represented by a non-nil error.

Wrapping adds context:

```go
fmt.Errorf(
    "patch PlatformApp status: %w",
    err,
)
```

Callers can still inspect wrapped errors using:

```go
errors.Is(...)
errors.As(...)
```

## 26. Practical success-state verification

Inspect generation and readiness:

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o jsonpath='generation={.metadata.generation}{"\n"}observedGeneration={.status.observedGeneration}{"\n"}readyReplicas={.status.readyReplicas}{"\n"}'
```

Inspect conditions:

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}{" reason="}{.reason}{" message="}{.message}{"\n"}{end}'
```

Scale the desired application:

```bash
kubectl patch platformapp nginx-demo \
  -n platformapp-demo \
  --type=merge \
  -p '{"spec":{"replicas":3}}'
```

During convergence, observedGeneration or readyReplicas may temporarily trail desired state.

After convergence:

```text
generation=observedGeneration
readyReplicas=desired replicas
Available=True
Progressing=False
Degraded=False
```

## 27. Practical failure test

We created a normal Deployment with an incompatible immutable selector:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: failure-demo
  namespace: platformapp-failure-test
spec:
  replicas: 1
  selector:
    matchLabels:
      existing: selector
  template:
    metadata:
      labels:
        existing: selector
    spec:
      containers:
      - name: existing
        image: nginx:1.27
```

We then created a PlatformApp using the same name:

```yaml
apiVersion: apps.fixnops.com/v1alpha1
kind: PlatformApp
metadata:
  name: failure-demo
  namespace: platformapp-failure-test
spec:
  image: nginx:1.27
  replicas: 1
  port: 80
```

The operator attempted to apply its managed selector. Kubernetes rejected the update because Deployment selectors are immutable.

The PlatformApp reported:

```text
Available=False
Progressing=False
Degraded=True
Reason=DeploymentReconciliationFailed
```

The message contained:

```text
field is immutable
```

No Service was created because reconciliation stopped after the Deployment failure.

This proved both failure reporting and ordered reconciliation behavior.

## 28. Why observedGeneration was still updated on failure

The failed PlatformApp reported:

```text
generation=1
observedGeneration=1
```

This does not mean reconciliation succeeded.

It means the controller processed generation 1 and recorded its failure.

The conditions communicate the result:

```text
Degraded=True
```

ObservedGeneration means “processed,” not “successful.”

## 29. Retry safety

Retries are safe because the controller is idempotent.

On retry:

- Existing correct resources remain correct
- Missing resources are created
- Incorrect mutable fields are updated
- The immutable selector conflict continues to return an error
- Status remains Degraded without unnecessary identical patches

After an administrator corrects the conflicting Deployment, a later retry can succeed and change:

```text
Degraded=True
```

to:

```text
Degraded=False
```

## 30. Status conflicts and concurrency

Kubernetes uses `resourceVersion` for optimistic concurrency.

Another update may occur between reading and patching a PlatformApp.

Using a merge patch based on the original deep copy reduces overwrite risk, but conflicts and transient errors can still occur.

Returning the error lets controller-runtime retrieve fresh state and retry.

Controllers should not assume their in-memory object remains current indefinitely.

## 31. Conditions versus events and logs

These mechanisms serve different purposes.

| Mechanism | Audience | Persistence |
|---|---|---|
| Status conditions | Users and automation | Stored on resource |
| Kubernetes Events | Operators and troubleshooting | Time-limited |
| Controller logs | Operations and debugging | External log retention |
| Metrics | Monitoring and alerting | Monitoring-system retention |

A production controller normally uses all four appropriately.

Our current implementation uses status, logs, and metrics. Custom Kubernetes Events can be added later if they provide useful operational value.

## 32. Common mistakes

### Returning success after a reconciliation failure

This prevents normal error retry behavior and can hide failures from controller metrics.

### Patching status every time

This can create unnecessary API traffic and reconciliation events.

### Updating status through the normal client endpoint

Use the status subresource client:

```go
r.Status()
```

### Treating observedGeneration as success

It only indicates which generation was processed. Conditions describe success or failure.

### Losing the original error

Status reporting failures should not replace the original reconciliation error.

### Using vague condition reasons

Reasons should be stable, machine-readable identifiers such as:

```text
DeploymentReconciliationFailed
```

Messages can contain human-readable details.

### Exposing secrets in condition messages

External service errors may contain sensitive information. Production operators should sanitize messages where necessary.

## 33. Interview questions and answers

### Why does the operator have status?

Status exposes observed state directly on the custom resource so users and automation do not need to inspect every managed child resource.

### What is the difference between generation and observedGeneration?

Generation identifies the current desired-state revision. ObservedGeneration identifies the revision most recently processed by the controller.

### Why use Kubernetes conditions?

Conditions provide a conventional, structured representation of resource state with type, status, reason, message, generation, and transition time.

### Why does the controller deep-copy before changing status?

The deep copy preserves the original status for comparison and merge-patch generation without sharing mutable pointer or slice data.

### Why compare old and new status?

It prevents unnecessary status patches, API calls, and reconcile events when nothing changed.

### Why use a patch instead of update?

A merge patch writes only calculated differences and reduces the risk of replacing unrelated fields.

### Why return an error after setting Degraded status?

Status informs users, while the returned error informs controller-runtime that reconciliation must be retried.

### What does controller-runtime do with returned errors?

It records reconciliation failure and requeues the resource through a rate-limited workqueue.

### Why use `errors.Join`?

It preserves both the original reconciliation error and a secondary status-patch error.

### Does observedGeneration equal generation mean success?

No. It means the generation was processed. Conditions show whether processing succeeded.

## 34. Practical verification checklist

Successful state:

```text
generation == observedGeneration
readyReplicas == desired replicas
Available=True
Progressing=False
Degraded=False
```

Progressing state:

```text
generation == observedGeneration
readyReplicas < desired replicas
Available=False
Progressing=True
Degraded=False
```

Failure state:

```text
generation == observedGeneration
Available=False
Progressing=False
Degraded=True
```

Controller behavior:

- Identical status is not patched
- Errors are returned
- Failed resources are retried
- Successful reconciliation clears Degraded state
- Status messages identify the failed component

## 35. Chapter summary

Status turns the PlatformApp from a desired-state document into an observable Kubernetes API.

The controller:

1. Calculates readiness from the Deployment.
2. Records the processed generation.
3. Maintains Available, Progressing, and Degraded conditions.
4. Avoids unchanged status writes.
5. Patches the status subresource.
6. Records failure details.
7. Returns errors for rate-limited retries.
8. Preserves multiple errors when both reconciliation and status reporting fail.

The next chapter explains how envtest and controller behavior tests verify resource creation, updates, ownership, idempotency, status, and failure paths.