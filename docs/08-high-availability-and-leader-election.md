# High Availability and Leader Election

This chapter explains how the PlatformApp Operator was changed from a
single-instance controller into a highly available controller deployment.

It covers:

- Why an Operator needs multiple replicas
- Why only one replica should actively reconcile resources
- Kubernetes leader election
- Lease objects
- Active and standby controller instances
- Leader failure and automatic failover
- Pod anti-affinity
- PodDisruptionBudgets
- Rolling updates
- Practical verification commands
- Production considerations
- Interview questions and answers

---

## 1. Why an Operator Needs High Availability

Our Operator continuously watches `PlatformApp` resources and reconciles their
desired state.

For every `PlatformApp`, it manages:

- A Kubernetes Deployment
- A Kubernetes Service
- Status information and conditions

Initially, the Operator Deployment had only one replica:

```yaml
spec:
  replicas: 1
```

This works for development, but it creates a single point of failure.

If the only Operator Pod stops because of:

- A process crash
- A node failure
- A Kubernetes upgrade
- A voluntary node drain
- A container restart
- A rolling deployment
- Resource pressure

then no controller is available to process changes until Kubernetes recreates
the Pod.

The existing application Pods generally continue running because Deployments,
Services, and Pods are stored and managed by Kubernetes.

However, while the Operator is unavailable:

- New `PlatformApp` resources are not reconciled
- Changes to existing `PlatformApp` resources are not processed
- Deleted child resources are not recreated
- Status is not refreshed
- Drift is not corrected

High availability reduces this controller downtime.

---

## 2. Application Availability Versus Operator Availability

The PlatformApp Operator and the applications it manages are different
workloads.

The Operator manages application resources, but it is not in the application
request path.

For example:

```text
Client -> Service -> Application Pod
```

The Operator does not receive the application's HTTP traffic.

Its role is approximately:

```text
PlatformApp resource -> Operator -> Deployment and Service
```

Therefore, if the Operator temporarily stops:

- Existing application Pods can continue running
- Existing Services can continue routing traffic
- Kubernetes Deployments can continue maintaining their existing replicas
- New desired-state changes are not reconciled until the Operator returns

This distinction is important in production troubleshooting and interviews.

---

## 3. Scaling the Operator to Two Replicas

We changed the controller manager Deployment from:

```yaml
replicas: 1
```

to:

```yaml
replicas: 2
```

The change was made in:

```text
config/manager/manager.yaml
```

The relevant section is:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
  namespace: system
spec:
  replicas: 2
```

Kustomize later transforms the name and namespace into:

```text
Deployment:
platformapp-operator-controller-manager

Namespace:
platformapp-operator-system
```

Running two replicas provides two controller manager processes.

However, simply running two replicas is not enough.

Without coordination, both replicas could attempt to reconcile the same
`PlatformApp` resources.

---

## 4. The Problem With Multiple Active Controllers

Suppose two Operator Pods are running:

```text
operator-pod-a
operator-pod-b
```

Both Pods watch the same Kubernetes API resources.

If they both actively reconcile the same event, they may simultaneously try to:

- Create the same Deployment
- Update the same Service
- Update the same status
- Correct the same drift
- Communicate with the same external system

Kubernetes optimistic concurrency and idempotent reconciliation reduce some
risks, but multiple active controllers can still cause:

- Conflicting updates
- Duplicate external operations
- Excessive API calls
- Status update conflicts
- Difficult troubleshooting
- Race conditions

For external resources, this could be more serious.

For example, two active controller instances could both attempt to create an
EC2 instance unless the design includes strong idempotency and coordination.

The usual controller-runtime pattern is therefore:

- Run multiple controller manager replicas
- Elect exactly one active leader
- Keep other replicas available as standby instances

---

## 5. What Leader Election Means

Leader election selects one controller manager Pod as the active leader.

The leader:

- Starts controller workers
- Processes reconciliation events
- Creates and updates managed resources
- Updates custom-resource status

The non-leader replicas:

- Remain running
- Keep participating in leader election
- Wait for leadership
- Do not run the controller reconciliation workers

The model is active-standby, not active-active.

With two replicas:

```text
Pod A: Leader and active reconciler
Pod B: Standby and waiting for leadership
```

If Pod A fails:

```text
Pod B: Acquires leadership and starts reconciling
New Pod: Created by the Deployment and becomes standby
```

---

## 6. Enabling Leader Election

Kubebuilder generated leader-election support in `cmd/main.go`.

The manager options include:

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    Scheme:                 scheme,
    Metrics:                metricsServerOptions,
    WebhookServer:          webhookServer,
    HealthProbeBindAddress: probeAddr,
    LeaderElection:         enableLeaderElection,
    LeaderElectionID:       "01670a09.fixnops.com",
})
```

The important fields are:

```go
LeaderElection:   enableLeaderElection
LeaderElectionID: "01670a09.fixnops.com"
```

`enableLeaderElection` is populated from this command-line option:

```go
flag.BoolVar(
    &enableLeaderElection,
    "leader-elect",
    false,
    "Enable leader election for controller manager...",
)
```

The deployed manager container receives:

```yaml
args:
- --leader-elect
```

Therefore, leader election is enabled when the Operator runs inside the
cluster.

When we ran the manager locally without `--leader-elect`, leader election was
disabled.

---

## 7. The Leader Election ID

The configured leader-election ID is:

```text
01670a09.fixnops.com
```

It uniquely identifies this controller manager's election.

Controller-runtime uses this value when creating the Kubernetes coordination
resource used for leadership.

Different controllers should use different leader-election IDs.

If unrelated controllers accidentally use the same ID in the same namespace,
they could compete in the same election even though they are different
applications.

The ID should therefore be:

- Stable between releases
- Unique to the Operator
- Kept unchanged unless there is a deliberate migration plan

---

## 8. Kubernetes Lease Object

Modern Kubernetes leader election normally uses a `Lease` resource from:

```text
coordination.k8s.io/v1
```

The PlatformApp Operator created a Lease named:

```text
01670a09.fixnops.com
```

in:

```text
platformapp-operator-system
```

Inspect it with:

```bash
kubectl get lease \
  -n platformapp-operator-system
```

Inspect the specific Lease:

```bash
kubectl get lease \
  01670a09.fixnops.com \
  -n platformapp-operator-system \
  -o yaml
```

Important fields can include:

```yaml
spec:
  acquireTime: ...
  holderIdentity: ...
  leaseDurationSeconds: ...
  renewTime: ...
```

The most useful field is:

```text
spec.holderIdentity
```

It identifies the current leader.

Use:

```bash
kubectl get lease \
  01670a09.fixnops.com \
  -n platformapp-operator-system \
  -o jsonpath='holderIdentity={.spec.holderIdentity}{"\n"}renewTime={.spec.renewTime}{"\n"}'
```

---

## 9. How Lease-Based Election Works

The general process is:

1. All Operator replicas start.
2. Each replica attempts to acquire the same Lease.
3. One replica successfully becomes the Lease holder.
4. That replica starts the controller workers.
5. The leader periodically renews the Lease.
6. Standby replicas continue observing the Lease.
7. If renewal stops long enough, another replica acquires it.
8. The new leader starts its controller workers.

The Kubernetes API server acts as the shared coordination system.

The Pods do not need to communicate with each other directly.

They coordinate by reading and updating the same Lease object.

---

## 10. Evidence From Our Operator Logs

When the deployed Operator started, its logs showed:

```text
Attempting to acquire leader lease...
```

Then one Pod logged:

```text
Successfully acquired lease
```

It subsequently started the controller:

```text
Starting Controller
Starting workers
```

These messages prove that the Pod:

1. Participated in leader election
2. Acquired the Lease
3. Became active
4. Started the PlatformApp controller workers

A standby Pod may show that it is attempting to acquire the Lease but will not
start the reconciliation workers until it becomes leader.

View logs for all controller manager Pods:

```bash
kubectl logs \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  --prefix=true \
  --tail=200
```

Search specifically for leader-election messages:

```bash
kubectl logs \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  --prefix=true \
  --tail=300 \
  | grep -E \
    'Attempting to acquire leader lease|Successfully acquired lease|became leader|Starting workers'
```

---

## 11. Leader Election Is Not Load Balancing

Leader election does not divide `PlatformApp` resources between two Pods.

It does not mean:

```text
Pod A reconciles half of the resources
Pod B reconciles the other half
```

Instead:

```text
Pod A is active
Pod B is standby
```

Only the active leader runs the controller workers.

This is important because scaling the controller Deployment to two replicas
does not double reconciliation throughput when leader election is enabled.

To increase reconciliation concurrency inside the active controller, a
different setting such as maximum concurrent reconciles would be used.

That is separate from controller high availability.

---

## 12. Leader Election Does Not Replace Idempotency

Even with leader election, the reconciliation function must remain idempotent.

Idempotency means that repeating reconciliation with the same desired state
produces the same final result without unnecessary side effects.

Leader changes can occur after partial work.

For example:

1. Leader A starts reconciliation.
2. Leader A creates the Deployment.
3. Leader A stops before creating the Service.
4. Leader B becomes active.
5. Leader B reconciles the same `PlatformApp`.
6. Leader B observes the Deployment and creates the missing Service.

Because our controller uses `CreateOrUpdate`, it can safely resume from the
current cluster state.

Leader election reduces concurrent execution, while idempotency makes retries
and failover safe.

Both are required.

---

## 13. Pod Anti-Affinity

Running two controller Pods does not guarantee that Kubernetes schedules them
on different nodes.

Without scheduling guidance, both replicas could be placed on one node.

If that node fails, both Operator Pods would become unavailable at the same
time.

We added preferred pod anti-affinity in:

```text
config/manager/manager.yaml
```

The configuration is:

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            control-plane: controller-manager
            app.kubernetes.io/name: platformapp-operator
        topologyKey: kubernetes.io/hostname
```

This tells the scheduler:

> Prefer placing matching controller manager Pods on different Kubernetes
> nodes.

---

## 14. Understanding the Anti-Affinity Fields

### `podAntiAffinity`

This requests separation from other matching Pods.

### `preferredDuringSchedulingIgnoredDuringExecution`

This is a scheduling preference, not a strict requirement.

Kubernetes tries to satisfy it, but the Pod may still be scheduled on the same
node if necessary.

### `weight: 100`

Weights range from 1 through 100.

A weight of 100 gives this scheduling preference the highest scoring
importance.

It does not turn the preference into a hard requirement.

### `labelSelector`

This identifies the Pods that should be kept apart:

```yaml
matchLabels:
  control-plane: controller-manager
  app.kubernetes.io/name: platformapp-operator
```

### `topologyKey`

We used:

```yaml
topologyKey: kubernetes.io/hostname
```

This means the scheduling topology is the Kubernetes node hostname.

The scheduler prefers different hostnames for the Operator replicas.

---

## 15. Preferred Versus Required Anti-Affinity

Kubernetes supports two broad anti-affinity approaches.

### Preferred anti-affinity

```yaml
preferredDuringSchedulingIgnoredDuringExecution:
```

Advantages:

- Improves availability when multiple nodes exist
- Still allows scheduling in small or degraded clusters
- Suitable for development clusters such as k3d
- Avoids leaving the Operator permanently Pending when only one node is usable

Trade-off:

- Does not guarantee node separation

### Required anti-affinity

```yaml
requiredDuringSchedulingIgnoredDuringExecution:
```

Advantages:

- Enforces separation across the selected topology
- Provides a stronger availability guarantee

Trade-off:

- A replica remains Pending if Kubernetes cannot find a valid different node

For our learning cluster, preferred anti-affinity was the practical choice.

In a production cluster, the choice depends on:

- Number of schedulable nodes
- Failure-domain design
- Availability requirements
- Upgrade behavior
- Capacity planning

---

## 16. Verifying Pod Distribution

We verified the Operator Pods with:

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  -o wide
```

The `NODE` column shows where each Pod is running.

In our k3d cluster, the replicas were distributed between:

```text
k3d-operator-lab-server-0
k3d-operator-lab-agent-0
```

This demonstrated that the scheduler satisfied the preferred anti-affinity.

Verify the rendered anti-affinity:

```bash
kubectl get deployment \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  -o jsonpath='replicas={.spec.replicas}{"\n"}antiAffinityWeight={.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight}{"\n"}topologyKey={.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey}{"\n"}'
```

Our result showed:

```text
replicas=2
antiAffinityWeight=100
topologyKey=kubernetes.io/hostname
```

---

## 17. PodDisruptionBudget

We added:

```text
config/manager/poddisruptionbudget.yaml
```

Its content is:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: controller-manager
  namespace: system
  labels:
    control-plane: controller-manager
    app.kubernetes.io/name: platformapp-operator
    app.kubernetes.io/managed-by: kustomize
spec:
  minAvailable: 1
  selector:
    matchLabels:
      control-plane: controller-manager
      app.kubernetes.io/name: platformapp-operator
```

It was added to:

```text
config/manager/kustomization.yaml
```

using:

```yaml
resources:
- manager.yaml
- poddisruptionbudget.yaml
```

Kustomize transforms the name and namespace during rendering.

The installed PodDisruptionBudget becomes:

```text
platformapp-operator-controller-manager
```

in:

```text
platformapp-operator-system
```

---

## 18. What `minAvailable: 1` Means

The PDB contains:

```yaml
spec:
  minAvailable: 1
```

This means Kubernetes should preserve at least one healthy matching Operator
Pod during voluntary disruptions.

Examples of voluntary disruptions include:

- Administratively draining a node
- Cluster maintenance using eviction
- Some automated node-management operations

With two healthy replicas:

```text
currentHealthy = 2
desiredHealthy = 1
allowedDisruptions = 1
```

Kubernetes can voluntarily evict one Operator Pod while keeping the other
available.

---

## 19. What a PodDisruptionBudget Does Not Protect Against

A PodDisruptionBudget does not prevent every failure.

It does not guarantee protection from:

- Hardware failure
- Kernel crash
- Power loss
- Direct Pod deletion
- Container process crash
- Node network failure
- Force deletion
- Incorrect configuration
- Resource exhaustion

A PDB primarily influences eviction-based voluntary disruption workflows.

It also does not create replicas.

The Deployment creates and maintains the replicas.

The PDB controls how many matching Pods may be voluntarily disrupted at once.

---

## 20. Verifying the PodDisruptionBudget

Use:

```bash
kubectl get poddisruptionbudget \
  -n platformapp-operator-system
```

Inspect the important fields:

```bash
kubectl get poddisruptionbudget \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  -o jsonpath='minAvailable={.spec.minAvailable}{"\n"}currentHealthy={.status.currentHealthy}{"\n"}desiredHealthy={.status.desiredHealthy}{"\n"}allowedDisruptions={.status.disruptionsAllowed}{"\n"}'
```

Our result showed:

```text
minAvailable=1
currentHealthy=2
desiredHealthy=1
allowedDisruptions=1
```

This confirmed that one healthy Operator Pod could be voluntarily disrupted
while preserving one available Pod.

---

## 21. Why We Need Both Leader Election and a PDB

Leader election and PodDisruptionBudget solve different problems.

| Feature | Purpose |
|---|---|
| Deployment replicas | Runs more than one Operator Pod |
| Leader election | Ensures only one replica actively reconciles |
| Pod anti-affinity | Prefers replicas on separate nodes |
| PodDisruptionBudget | Limits simultaneous voluntary disruption |
| Readiness probe | Prevents an unhealthy Pod from being considered ready |
| Liveness probe | Allows Kubernetes to restart an unhealthy process |
| Idempotent reconciliation | Makes retries and leader transitions safe |

No single one of these features provides complete availability.

Production reliability comes from combining them.

---

## 22. Rolling Updates

When the Operator Deployment changes, Kubernetes performs a rolling update.

During the update, it may temporarily show more Pods than the desired replica
count.

For example, with:

```yaml
replicas: 2
```

you might temporarily observe three Pods:

- Two Pods from the old ReplicaSet
- One Pod from the new ReplicaSet

This can occur because the default rolling-update strategy permits a surge.

The Deployment output may briefly show values such as:

```text
READY       2/2
UP-TO-DATE  1
AVAILABLE   2
```

This means:

- Two Pods are currently available
- Only one belongs to the latest Pod template
- Kubernetes is still completing the rolling update

It is not an error.

Always wait for rollout completion:

```bash
kubectl rollout status \
  deployment/platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  --timeout=120s
```

Then verify:

```bash
kubectl get deployment,replicaset,pods \
  -n platformapp-operator-system \
  -o wide
```

The old ReplicaSet should eventually have zero desired replicas.

---

## 23. Practical Leader-Failover Test

A useful HA test is to identify and delete the current leader.

### Step 1: List Operator Pods

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  -o wide
```

### Step 2: Identify the Lease holder

```bash
kubectl get lease \
  01670a09.fixnops.com \
  -n platformapp-operator-system \
  -o jsonpath='{.spec.holderIdentity}{"\n"}'
```

The holder identity normally contains the leader Pod identity.

### Step 3: Record application health

```bash
kubectl get platformapps,deployments,pods \
  -n platformapp-demo
```

### Step 4: Delete only the leader Pod

Use the exact Pod name returned by the Lease and Pod listing:

```bash
kubectl delete pod \
  <leader-pod-name> \
  -n platformapp-operator-system
```

Do not guess the Pod name.

### Step 5: Watch Operator Pods

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  -w
```

The Deployment should create a replacement Pod.

### Step 6: Check the new Lease holder

```bash
kubectl get lease \
  01670a09.fixnops.com \
  -n platformapp-operator-system \
  -o jsonpath='{.spec.holderIdentity}{"\n"}'
```

### Step 7: Inspect failover logs

```bash
kubectl logs \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  --prefix=true \
  --since=10m \
  | grep -E \
    'Successfully acquired lease|became leader|Starting workers'
```

### Step 8: Verify the application remained available

```bash
kubectl get platformapps,deployments,pods \
  -n platformapp-demo
```

The application workload should remain running because it is independent of
the controller manager's process lifecycle.

---

## 24. Testing Reconciliation After Failover

A stronger test is to change a `PlatformApp` after leadership transfers.

For example:

```bash
kubectl patch platformapp nginx-demo \
  -n platformapp-demo \
  --type=merge \
  -p '{"spec":{"replicas":3}}'
```

Watch the Deployment:

```bash
kubectl get deployment nginx-demo \
  -n platformapp-demo \
  -w
```

Check the custom-resource status:

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o jsonpath='generation={.metadata.generation}{"\n"}observedGeneration={.status.observedGeneration}{"\n"}readyReplicas={.status.readyReplicas}{"\n"}'
```

Successful processing proves that the new leader is actively reconciling.

Restore the previous replica count if necessary:

```bash
kubectl patch platformapp nginx-demo \
  -n platformapp-demo \
  --type=merge \
  -p '{"spec":{"replicas":2}}'
```

---

## 25. Leader Election RBAC

The Operator needs permission to use the coordination resource required for
leader election.

Kubebuilder generated:

```text
config/rbac/leader_election_role.yaml
```

and:

```text
config/rbac/leader_election_role_binding.yaml
```

These resources are namespace-scoped.

They allow the controller manager ServiceAccount to work with leader-election
resources in the Operator namespace.

This is separate from the ClusterRole used for watching `PlatformApp`
resources and managing application Deployments and Services.

The separation follows least-privilege principles:

- ClusterRole: permissions needed across watched namespaces
- Namespaced Role: permissions needed only inside the Operator namespace

---

## 26. Inspecting Leader-Election RBAC

Inspect the Role:

```bash
kubectl get role \
  platformapp-operator-leader-election-role \
  -n platformapp-operator-system \
  -o yaml
```

Inspect the RoleBinding:

```bash
kubectl get rolebinding \
  platformapp-operator-leader-election-rolebinding \
  -n platformapp-operator-system \
  -o yaml
```

Confirm the controller ServiceAccount can work with Leases:

```bash
OPERATOR_SA='system:serviceaccount:platformapp-operator-system:platformapp-operator-controller-manager'
```

Then:

```bash
kubectl auth can-i get leases.coordination.k8s.io \
  --as="$OPERATOR_SA" \
  -n platformapp-operator-system
```

```bash
kubectl auth can-i create leases.coordination.k8s.io \
  --as="$OPERATOR_SA" \
  -n platformapp-operator-system
```

```bash
kubectl auth can-i update leases.coordination.k8s.io \
  --as="$OPERATOR_SA" \
  -n platformapp-operator-system
```

Expected answers should be:

```text
yes
```

Unset the temporary shell variable afterward:

```bash
unset OPERATOR_SA
```

---

## 27. Health Probes With Multiple Replicas

Each Operator Pod exposes:

```text
/healthz
/readyz
```

The liveness probe checks whether the controller manager process is alive.

The readiness probe checks whether the Pod is ready.

These probes operate independently on every replica.

Important distinction:

- A standby replica can still be healthy and ready
- Readiness does not necessarily mean the Pod is the current leader
- Leader election controls controller-worker activity
- Probes control Pod health and readiness lifecycle

This is expected behavior.

The standby Pod must remain alive and ready so it can quickly participate in
failover.

---

## 28. Metrics With Multiple Replicas

Each controller manager Pod exposes its own process and controller-runtime
metrics.

The metrics Service selects both Operator Pods.

Depending on how metrics are scraped, a monitoring system may collect metrics
from both replicas.

Some metrics are meaningful per Pod, such as:

- Go process memory
- CPU usage
- Process restarts
- Leader-election behavior
- Controller-runtime worker information

Only the leader should show active reconciliation work while it holds
leadership.

Monitoring systems should retain Pod labels so metrics from separate replicas
are distinguishable.

---

## 29. Availability Limitations in the Learning Cluster

Our k3d cluster uses containers to simulate Kubernetes nodes.

It is useful for learning:

- Multi-node scheduling
- Leader election
- Pod replacement
- Anti-affinity behavior
- PDB behavior
- Deployment rollout behavior

However, it is not equivalent to a production cluster with independent
physical or virtual failure domains.

Both k3d nodes ultimately run through the same local Docker environment and the
same Mac.

Therefore, the lab demonstrates Kubernetes behavior but does not provide true
infrastructure-level high availability.

A production design would normally consider:

- Independent worker nodes
- Multiple availability zones
- Control-plane availability
- Persistent API-server and etcd availability
- Network failure domains
- Registry availability
- Monitoring availability
- Pod topology spread
- Capacity during node maintenance

---

## 30. Current HA Architecture

Our current Operator architecture is:

```text
Operator Deployment
    replicas: 2

Replica 1
    controller manager
    health endpoint
    metrics endpoint
    leader-election participant

Replica 2
    controller manager
    health endpoint
    metrics endpoint
    leader-election participant

Shared coordination
    Kubernetes Lease

Only Lease holder
    starts controller workers
    reconciles PlatformApps

Standby replica
    waits to acquire leadership
```

Supporting availability controls include:

```text
Preferred pod anti-affinity
PodDisruptionBudget with minAvailable: 1
Liveness and readiness probes
Resource requests and limits
Restricted security context
Idempotent reconciliation
```

---

## 31. Troubleshooting: Both Pods Are Running but Only One Reconciles

This is normal when leader election is enabled.

Check:

```bash
kubectl get lease \
  01670a09.fixnops.com \
  -n platformapp-operator-system \
  -o yaml
```

Then inspect logs from both Pods.

Only the leader should start the controller workers.

The standby Pod should continue attempting or waiting to acquire leadership.

---

## 32. Troubleshooting: Neither Pod Becomes Leader

Check the following.

### Operator Pod logs

```bash
kubectl logs \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  --prefix=true \
  --tail=200
```

### Lease permissions

```bash
kubectl auth can-i create leases.coordination.k8s.io \
  --as='system:serviceaccount:platformapp-operator-system:platformapp-operator-controller-manager' \
  -n platformapp-operator-system
```

### ServiceAccount

```bash
kubectl get deployment \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  -o jsonpath='{.spec.template.spec.serviceAccountName}{"\n"}'
```

### RoleBinding subjects

```bash
kubectl get rolebinding \
  platformapp-operator-leader-election-rolebinding \
  -n platformapp-operator-system \
  -o yaml
```

### Existing Lease

```bash
kubectl get lease \
  -n platformapp-operator-system
```

Common causes include:

- Missing Lease RBAC
- Incorrect ServiceAccount
- RoleBinding referencing the wrong ServiceAccount
- API-server connectivity problems
- Incorrect namespace
- Multiple unrelated controllers using the same election ID

---

## 33. Troubleshooting: Both Replicas Run on One Node

Preferred anti-affinity is not a hard guarantee.

Check:

```bash
kubectl get nodes
```

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  -o wide
```

```bash
kubectl describe pod \
  -n platformapp-operator-system \
  -l control-plane=controller-manager
```

Possible reasons include:

- Only one schedulable node
- Resource pressure on another node
- Node taints
- Node selectors
- Insufficient CPU or memory
- Scheduler constraints
- Preferred anti-affinity was outweighed by other scheduling needs

If strict separation is required, production architects may consider required
anti-affinity or topology spread constraints.

That decision must be supported by sufficient cluster capacity.

---

## 34. Troubleshooting: PodDisruptionBudget Blocks a Drain

This can be expected.

If only one healthy matching Pod remains and the PDB requires:

```yaml
minAvailable: 1
```

Kubernetes should not voluntarily evict that final healthy Pod.

Inspect:

```bash
kubectl get poddisruptionbudget \
  -n platformapp-operator-system
```

```bash
kubectl describe poddisruptionbudget \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system
```

Then investigate why the second replica is unavailable before overriding the
protection.

Do not immediately delete the PDB without understanding the availability
impact.

---

## 35. Production Improvements Beyond the Current Lab

The current HA setup is a strong baseline.

Additional production decisions may include:

- Three replicas instead of two
- Required anti-affinity
- Topology spread across zones
- Explicit rolling-update values
- PriorityClass
- More carefully measured resource requests
- Alerts for loss of leader
- Alerts for unavailable replicas
- Alerts for reconciliation failures
- Alerts for repeated leader changes
- NetworkPolicy
- Dedicated namespaces
- Namespace-scoped versus cluster-scoped watching
- Disaster-recovery procedures
- Multi-cluster deployment strategy

These should be based on actual service-level objectives rather than added
without operational justification.

---

## 36. Why Two Replicas Are a Reasonable Starting Point

Two replicas provide:

- One active leader
- One warm standby
- Automatic replacement through the Deployment
- One allowed voluntary disruption with `minAvailable: 1`
- Low resource overhead for the learning environment

A limitation is that two replicas do not provide the same failure-domain
flexibility as three replicas.

Unlike etcd quorum, controller-runtime leader election does not require a
majority of controller Pods.

The Kubernetes API and Lease provide coordination.

Therefore, two replicas can provide active-standby controller availability.

---

## 37. Relationship to Kubernetes Control-Plane Availability

Operator leader election depends on the Kubernetes API server.

If the Kubernetes control plane is unavailable:

- The leader cannot renew its Lease
- Standby replicas cannot reliably acquire the Lease
- Reconciliation cannot read or update Kubernetes resources
- Custom-resource updates cannot be processed

Application Pods may continue running temporarily, but control-plane operations
will be unavailable.

Operator HA does not replace Kubernetes control-plane HA.

It assumes the underlying Kubernetes API is available.

---

## 38. Important Go and Controller-Runtime Concepts

The high-availability behavior is mostly provided by controller-runtime.

The essential Go configuration is:

```go
ctrl.Options{
    LeaderElection:   enableLeaderElection,
    LeaderElectionID: "01670a09.fixnops.com",
}
```

We did not implement our own distributed locking algorithm.

Controller-runtime handles:

- Lease acquisition
- Lease renewal
- Leadership loss
- Starting leader-elected runnables
- Coordinating manager startup

This is an important engineering principle:

> Use the established controller-runtime leader-election implementation instead
> of building custom distributed coordination logic.

---

## 39. Interview Explanation

A concise interview explanation is:

> I deployed the controller manager with two replicas for availability and
> enabled controller-runtime leader election using a Kubernetes Lease. Only the
> Lease holder starts the reconciliation workers, while the other replica
> remains a warm standby. I added preferred pod anti-affinity across hostnames
> so the replicas are normally scheduled on different nodes, and a
> PodDisruptionBudget with `minAvailable: 1` to protect against simultaneous
> voluntary disruption. I verified failover by deleting the leader Pod,
> observing the standby acquire the Lease, and confirming that the managed
> application remained available and reconciliation continued.

---

## 40. Common Interview Questions

### Why not allow both Operator replicas to reconcile?

Multiple active replicas can produce concurrent writes, duplicate external
operations, status conflicts, and race conditions. Leader election selects one
active reconciler while retaining standby availability.

### What Kubernetes resource is used for leader election?

A `Lease` from the `coordination.k8s.io/v1` API.

### Where is the Lease stored?

In the Operator namespace:

```text
platformapp-operator-system
```

### What happens when the leader Pod fails?

It stops renewing the Lease. A standby replica acquires leadership and starts
the controller workers. The Deployment also creates a replacement Pod.

### Does leader election distribute workload between replicas?

No. This setup is active-standby, not load-balanced reconciliation.

### Does a PDB protect against all failures?

No. It primarily controls voluntary evictions. It does not prevent hardware
failure, process crashes, direct deletion, or forced disruption.

### Why use preferred anti-affinity?

It encourages node separation while still allowing scheduling in a small or
degraded cluster.

### Why is idempotency still required?

A leader can fail after completing only part of a reconciliation. The new
leader must safely observe the current state and continue without creating
duplicate or inconsistent resources.

### Will applications stop if the Operator stops?

Existing application resources normally continue running because the Operator
is not in the request path. However, new changes, drift correction, and status
updates pause until an Operator becomes active.

### Is a ready standby Pod also reconciling?

No. A standby Pod may be healthy and ready while it waits for leadership.
Readiness and leadership are separate concepts.

---

## 41. Practical Verification Checklist

### Deployment replicas

```bash
kubectl get deployment \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  -o jsonpath='{.spec.replicas}{"\n"}'
```

Expected:

```text
2
```

### Ready Operator Pods

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  -o wide
```

Expected:

- Two Running Pods
- Two Ready Pods
- Preferably placed on different nodes

### Leader Lease

```bash
kubectl get lease \
  01670a09.fixnops.com \
  -n platformapp-operator-system \
  -o jsonpath='{.spec.holderIdentity}{"\n"}'
```

Expected:

- One holder identity

### PDB

```bash
kubectl get poddisruptionbudget \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system
```

Expected:

```text
MIN AVAILABLE: 1
```

### Anti-affinity

```bash
kubectl get deployment \
  platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  -o jsonpath='{.spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey}{"\n"}'
```

Expected:

```text
kubernetes.io/hostname
```

### Controller activity

```bash
kubectl logs \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  --prefix=true \
  --tail=300 \
  | grep -E \
    'Successfully acquired lease|Starting workers|Reconciling PlatformApp'
```

### Application health

```bash
kubectl get platformapps,deployments,pods \
  -n platformapp-demo
```

---

## 42. Files Involved

The main files involved in high availability are:

```text
cmd/main.go
config/manager/manager.yaml
config/manager/poddisruptionbudget.yaml
config/manager/kustomization.yaml
config/rbac/leader_election_role.yaml
config/rbac/leader_election_role_binding.yaml
config/default/kustomization.yaml
```

Their responsibilities are:

| File | Responsibility |
|---|---|
| `cmd/main.go` | Configures controller-runtime leader election |
| `manager.yaml` | Sets two replicas, arguments, probes, resources, security and anti-affinity |
| `poddisruptionbudget.yaml` | Preserves one healthy replica during voluntary disruption |
| `manager/kustomization.yaml` | Includes manager and PDB resources |
| `leader_election_role.yaml` | Provides namespace-scoped election permissions |
| `leader_election_role_binding.yaml` | Assigns election permissions to the Operator ServiceAccount |
| `default/kustomization.yaml` | Composes the complete deployment configuration |

---

## 43. What We Learned

By completing this stage, we learned that production controller availability
requires several cooperating mechanisms:

1. Multiple controller manager replicas provide redundancy.
2. Leader election prevents simultaneous active reconciliation.
3. Kubernetes Leases coordinate leadership.
4. A standby replica can take over when the leader fails.
5. Anti-affinity reduces the chance that replicas share one node.
6. A PodDisruptionBudget protects against excessive voluntary disruption.
7. Health probes and leader election solve different problems.
8. Existing applications normally continue running during controller failover.
9. Idempotent reconciliation makes leadership transitions safe.
10. Operator HA still depends on Kubernetes API availability.

The PlatformApp Operator now has a practical active-standby high-availability
design suitable as a production-oriented baseline.