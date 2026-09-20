# Chapter 5: Controller Testing with Envtest

## Learning objectives

After this chapter, you should be able to explain:

- Why Kubernetes controllers require more than ordinary unit tests
- What controller-runtime envtest provides
- What envtest does not provide
- How Ginkgo and Gomega structure tests
- How the test suite starts and stops its API server
- How reconciliation is invoked during tests
- How resource creation and updates are verified
- How ownership and status are tested
- How idempotency is tested
- How Deployment and Service failures are tested
- How to reuse this test structure for another operator

## 1. Why controller testing is different

A Kubernetes controller interacts with:

- Kubernetes API objects
- CRD validation
- Defaulting
- Resource versions
- Generation
- Status subresources
- Owner references
- API errors
- Optimistic concurrency

A simple test using only Go structs would not verify all these behaviors.

The PlatformApp controller tests use controller-runtime envtest to run a real Kubernetes API server and etcd locally.

## 2. Testing levels

A production operator normally uses several testing levels.

| Test level | Purpose |
|---|---|
| Go unit test | Tests small functions without Kubernetes |
| Fake-client test | Tests client interactions in memory |
| Envtest | Tests against API server and etcd |
| Integration test | Tests controller behavior in a real cluster |
| End-to-end test | Tests installation and user workflows |

Our current controller suite primarily uses envtest.

We also performed manual integration tests using k3d.

## 3. What envtest provides

Envtest starts:

- Kubernetes API server
- etcd

It supports:

- Installing CRDs
- API validation
- API defaulting
- Resource versions
- Generation behavior
- Status subresources
- Kubernetes CRUD operations
- API errors such as immutable-field failures

This makes envtest more realistic than testing only Go objects.

## 4. What envtest does not provide

Envtest does not start the complete Kubernetes control plane.

It does not normally provide:

- Scheduler
- Kubelet
- Deployment controller
- ReplicaSet controller
- Service controller
- Real Nodes
- Real Pods
- Container runtime
- Networking

Therefore, an envtest Deployment does not automatically create ReplicaSets or Pods.

This is why the test expects:

```text
readyReplicas=0
```

even after the Deployment object is successfully created.

The k3d tests verify actual Pod scheduling, Service endpoints, application connectivity, rollout readiness, and leader failover.

## 5. Ginkgo and Gomega

The test suite uses:

- Ginkgo v2
- Gomega

Ginkgo provides behavior-driven test structure.

Gomega provides assertions.

Imports use dot syntax:

```go
. "github.com/onsi/ginkgo/v2"
. "github.com/onsi/gomega"
```

This allows code such as:

```go
Describe(...)
Context(...)
BeforeEach(...)
It(...)
Expect(...)
```

without package prefixes.

Dot imports should be used carefully in normal Go code, but they are conventional in Ginkgo test files.

## 6. Test suite entry point

The standard Go test entry point is:

```go
func TestControllers(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Controller Suite")
}
```

`RegisterFailHandler(Fail)` connects Gomega assertion failures to Ginkgo.

`RunSpecs` executes the Ginkgo suite through Go's normal test runner.

Therefore, the tests still run using:

```bash
go test
```

or:

```bash
make test
```

## 7. Shared suite variables

The suite declares:

```go
var (
    ctx       context.Context
    cancel    context.CancelFunc
    testEnv   *envtest.Environment
    cfg       *rest.Config
    k8sClient client.Client
)
```

These variables are shared by the controller tests.

| Variable | Purpose |
|---|---|
| `ctx` | Context for API operations |
| `cancel` | Cancels the suite context |
| `testEnv` | Envtest environment |
| `cfg` | REST configuration for test API server |
| `k8sClient` | Kubernetes client connected to envtest |

## 8. BeforeSuite

`BeforeSuite` runs once before all test cases.

It configures logging:

```go
logf.SetLogger(
    zap.New(
        zap.WriteTo(GinkgoWriter),
        zap.UseDevMode(true),
    ),
)
```

Logs are written into Ginkgo output, which is especially useful when a test fails.

## 9. Registering the API scheme

The test suite registers PlatformApp:

```go
err = appsv1alpha1.AddToScheme(
    scheme.Scheme,
)
Expect(err).NotTo(HaveOccurred())
```

The client must know how to serialize and deserialize PlatformApp objects.

Built-in Kubernetes types already exist in the client-go scheme.

## 10. Loading CRDs

Envtest is configured with:

```go
testEnv = &envtest.Environment{
    CRDDirectoryPaths: []string{
        filepath.Join(
            "..",
            "..",
            "config",
            "crd",
            "bases",
        ),
    },
    ErrorIfCRDPathMissing: true,
}
```

Before tests begin, envtest installs the generated PlatformApp CRD.

`ErrorIfCRDPathMissing` prevents the suite from silently running without the CRD.

This ensures tests use the generated validation schema and status subresource.

## 11. Envtest binaries

Envtest requires local binaries such as:

- `kube-apiserver`
- `etcd`
- `kubectl`

The Makefile downloads version-matched binaries through `setup-envtest`.

The test target sets:

```text
KUBEBUILDER_ASSETS
```

to the binary directory before running `go test`.

The Kubernetes version is derived from the Kubernetes API dependency in `go.mod`.

In this project, the test setup selected Kubernetes 1.37 binaries.

## 12. Starting the test environment

The suite starts envtest:

```go
cfg, err = testEnv.Start()
Expect(err).NotTo(HaveOccurred())
Expect(cfg).NotTo(BeNil())
```

It then creates a client:

```go
k8sClient, err = client.New(
    cfg,
    client.Options{
        Scheme: scheme.Scheme,
    },
)
```

This client communicates with the temporary API server.

## 13. AfterSuite

After all tests complete:

```go
cancel()
err := testEnv.Stop()
Expect(err).NotTo(HaveOccurred())
```

This stops the API server and etcd processes.

Correct teardown prevents orphaned processes and port conflicts.

## 14. Test organization

The controller test begins with:

```go
var _ = Describe(
    "PlatformApp Controller",
    func() {
        Context(
            "When reconciling a PlatformApp",
            func() {
                // Tests
            },
        )
    },
)
```

The hierarchy communicates behavior:

```text
PlatformApp Controller
└── When reconciling a PlatformApp
    ├── creates expected resources
    ├── applies spec changes
    ├── remains idempotent
    ├── reports Deployment failures
    └── reports Service failures
```

## 15. Shared resource identity

Tests use:

```go
const (
    resourceName      = "test-resource"
    resourceNamespace = "default"
)
```

The combined identity is:

```go
types.NamespacedName{
    Name:      resourceName,
    Namespace: resourceNamespace,
}
```

This is the same form used by reconciliation requests and client `Get()` operations.

## 16. Expected labels

The test defines:

```go
expectedLabels := map[string]string{
    "app.kubernetes.io/name":       "platformapp",
    "app.kubernetes.io/instance":   resourceName,
    "app.kubernetes.io/managed-by": "platformapp-operator",
}
```

The same expected value is reused for:

- Deployment labels
- Deployment selector
- Pod template labels
- Service labels
- Service selector

This prevents duplicated literal maps across assertions.

## 17. BeforeEach

`BeforeEach` runs before every `It` block.

It creates a valid PlatformApp:

```go
Spec: appsv1alpha1.PlatformAppSpec{
    Image: "nginx:1.27",
    Port:  80,
}
```

Replicas is omitted intentionally. The CRD defaults it to one.

The setup also creates the reconciler:

```go
controllerReconciler = &PlatformAppReconciler{
    Client: k8sClient,
    Scheme: k8sClient.Scheme(),
}
```

## 18. Why the test resource must be valid

The original scaffold test created a PlatformApp without image or port.

After validation was added, the API server rejected it:

```text
spec.image must be at least one character
spec.port must be greater than or equal to one
```

The test was corrected to provide a valid spec.

This demonstrated that envtest applies CRD validation.

## 19. AfterEach cleanup

`AfterEach` deletes:

- Service
- Deployment
- Test ConfigMap
- PlatformApp

The cleanup handles NotFound:

```go
if errors.IsNotFound(err) {
    continue
}
```

This is required because not every failure test creates every object.

For example:

- Deployment failure does not create the Service
- Most tests do not create the ConfigMap

Tests should not fail merely because an optional test object is already absent.

## 20. Reconciliation helper

The suite defines:

```go
reconcilePlatformApp := func() {
    _, err := controllerReconciler.Reconcile(
        ctx,
        reconcile.Request{
            NamespacedName: typeNamespacedName,
        },
    )
    Expect(err).NotTo(HaveOccurred())
}
```

This reduces repeated setup in successful test cases.

The failure tests call `Reconcile()` directly because they expect an error.

## 21. Test 1: resource creation

The first test verifies that reconciliation creates:

- Deployment
- Service
- Initial PlatformApp status

It does not merely check that objects exist. It validates important fields.

### Deployment assertions

The test verifies:

- Labels
- Default replicas
- Selector
- Pod labels
- Container count
- Container name
- Image
- Port name
- Container port
- TCP protocol

### Service assertions

The test verifies:

- Labels
- Selector
- ClusterIP type
- Service port
- Target port
- TCP protocol

## 22. Owner-reference assertions

The test calls:

```go
metav1.IsControlledBy(
    deployment,
    platformApp,
)
```

and:

```go
metav1.IsControlledBy(
    service,
    platformApp,
)
```

This proves PlatformApp is the controlling owner.

Checking only object existence would miss lifecycle and watch-mapping defects.

## 23. Initial status assertions

Because envtest does not run the Deployment controller:

```text
readyReplicas=0
```

The test expects:

```text
Available=False
Reason=ReplicasNotReady
```

```text
Progressing=True
Reason=WaitingForReplicas
```

```text
Degraded=False
Reason=ReconciliationSucceeded
```

It also verifies:

```text
observedGeneration == generation
```

## 24. Test 2: desired-state update

The second test performs initial reconciliation and then changes:

```go
platformApp.Spec.Image = "nginx:1.28"
platformApp.Spec.Replicas = &updatedReplicas
```

It updates the PlatformApp through the API server:

```go
k8sClient.Update(ctx, platformApp)
```

The test confirms generation increased.

After a second reconciliation, it verifies:

- Deployment image changed
- Deployment replicas changed to three
- Status processed the new generation
- Status remains Progressing because envtest has no ready Pods

This proves the controller handles both create and update paths.

## 25. Test 3: idempotency

The idempotency test:

1. Reconciles once.
2. Retrieves PlatformApp, Deployment, and Service.
3. Records their resource versions.
4. Reconciles again without changing desired state.
5. Retrieves all objects again.
6. Compares resource versions.

Expected:

```text
resourceVersion before == resourceVersion after
```

If a resource version changed, the controller performed an unnecessary API write.

This test verifies:

- Stable Deployment is not updated
- Stable Service is not updated
- Identical PlatformApp status is not patched

Idempotency is a core controller requirement.

## 26. Resource versions

Kubernetes changes:

```text
metadata.resourceVersion
```

whenever an object is successfully modified.

Resource version is therefore useful for detecting unexpected writes.

It should not be treated as a numeric counter or parsed by application code. Tests only compare equality before and after reconciliation.

## 27. Test 4: Deployment failure

The test creates a Deployment with selector:

```yaml
matchLabels:
  existing: selector
```

The operator wants its standard PlatformApp labels.

Deployment selectors are immutable, so reconciliation fails when it attempts to change the selector.

The test expects:

- Reconcile returns an error
- Service is not created
- observedGeneration matches generation
- Available is False
- Progressing is False
- Degraded is True
- Degraded reason is `DeploymentReconciliationFailed`
- Degraded message is not empty

This verifies real Kubernetes API failure handling.

## 28. Why Service should not exist after Deployment failure

Reconciliation executes in this order:

```text
Deployment
    ↓
Service
    ↓
Status
```

If Deployment reconciliation fails, the method returns immediately through `handleReconcileError`.

Service reconciliation should not continue.

The test verifies this using a NotFound response.

## 29. Test 5: Service failure

The Service failure test creates a ConfigMap:

```text
test-resource-owner
```

It then creates a Service controlled by that ConfigMap.

The operator attempts:

```go
SetControllerReference(
    platformApp,
    service,
    scheme,
)
```

A Kubernetes object may have only one controlling owner.

Because the Service is already controlled by the ConfigMap, `SetControllerReference` returns an ownership error.

## 30. What the Service failure test proves

The test expects:

- Reconcile returns an error
- Deployment exists because its reconciliation succeeded first
- PlatformApp observedGeneration is current
- Available is False
- Progressing is False
- Degraded is True
- Degraded reason is `ServiceReconciliationFailed`
- Degraded message is not empty

This provides coverage for the second component-specific failure path.

## 31. Why use a real ownership conflict?

A weak failure test might manually construct an error without exercising controller behavior.

The ownership-conflict test uses:

- A real ConfigMap
- A real Service
- A real controller owner reference
- The actual `SetControllerReference` implementation
- The real reconciliation path

This produces a deterministic error without requiring a custom fake client.

## 32. Makefile test target

The test target is:

```make
test: manifests generate fmt vet setup-envtest
	KUBEBUILDER_ASSETS="..." go test ... -coverprofile cover.out
```

Before running tests, it:

1. Regenerates manifests.
2. Regenerates Go code.
3. Formats Go code.
4. Runs static analysis.
5. Installs envtest binaries.
6. Runs Go tests.
7. Writes coverage to `cover.out`.

This catches generated-code drift and formatting problems before test execution.

## 33. Static analysis

`go vet` checks suspicious Go constructs that may compile but indicate defects.

Examples include:

- Incorrect formatting directives
- Unreachable or malformed code patterns
- Misused struct tags
- Copying synchronization values

It complements tests but does not replace them.

## 34. Test coverage

After adding the Service failure test, controller coverage reached:

```text
91.1%
```

Coverage is useful for finding untested paths, but a high number alone does not prove test quality.

Important behaviors matter more than executing every line.

Our tests cover:

- Create path
- Update path
- Stable path
- Deployment failure path
- Service failure path
- Status success and failure
- Owner references

## 35. What remains outside envtest

The following behaviors were validated in k3d instead:

- Real Deployment rollout
- ReplicaSet creation
- Pod scheduling
- Container readiness
- Service EndpointSlices
- In-cluster network access
- Operator container startup
- ServiceAccount authentication
- RBAC authorization
- Health probes
- HTTPS metrics
- Multiple operator replicas
- Leader-election failover
- PodDisruptionBudget behavior

These could later be automated using the generated end-to-end test structure.

## 36. Testing pyramid for the next operator

For another operator, use this sequence:

### Unit tests

Test pure helper functions:

- Naming
- Label construction
- Condition calculation
- Cloud request construction

### Envtest

Test Kubernetes API behavior:

- CR creation and validation
- Reconciliation
- Managed resources
- Ownership
- Status
- Failure paths
- Idempotency

### End-to-end tests

Test complete runtime behavior:

- Installation
- Real workloads
- Networking
- RBAC
- Metrics
- Leader election
- Upgrades
- Uninstallation

External-resource operators should also use mocked provider APIs or dedicated test accounts.

## 37. Common testing mistakes

### Creating invalid fixtures unintentionally

Fixtures must satisfy the current CRD validation rules unless rejection is the behavior being tested.

### Checking only that Reconcile returned nil

A nil error does not prove that correct resources were created.

### Checking only object existence

Verify important fields, ownership, status, and update behavior.

### Ignoring cleanup

Leftover resources can make later tests pass or fail incorrectly.

### Assuming envtest creates Pods

Envtest does not run the full set of Kubernetes controllers.

### Testing only successful behavior

Production controllers need explicit failure-path tests.

### Using arbitrary fake errors where real API behavior is available

Real immutable-field and ownership conflicts provide stronger tests.

### Ignoring idempotency

A controller can be functionally correct while continuously writing unchanged resources.

## 38. Debugging failed tests

Run:

```bash
make test
```

For more detailed Ginkgo output:

```bash
KUBEBUILDER_ASSETS="$(
  ./bin/setup-envtest use 1.37 \
    --bin-dir ./bin \
    -p path
)" go test ./internal/controller \
  -ginkgo.v
```

Inspect the first failure rather than only the final summary.

Useful checks include:

- Does the fixture satisfy CRD validation?
- Is the API type registered in the scheme?
- Was the CRD regenerated?
- Did the previous test clean up?
- Is the expected namespace correct?
- Does envtest provide the controller behavior being asserted?
- Is status being retrieved again after reconciliation?

## 39. Reusable test checklist

For each new managed resource, test:

- Resource is created
- Name and namespace are correct
- Labels are correct
- Important spec fields are correct
- Owner reference is correct
- Desired-state updates are applied
- Stable reconciliation performs no writes
- API failures update status
- Reconcile returns failures
- Later resources are not created after an earlier required step fails

For status, test:

- observedGeneration
- Ready or observed counts
- Available condition
- Progressing condition
- Degraded condition
- Condition reasons
- Failure messages
- Unchanged status is not patched

## 40. Interview questions and answers

### What is envtest?

Envtest is a controller-runtime testing environment that starts a local Kubernetes API server and etcd so controllers can be tested against real Kubernetes API behavior.

### Is envtest a complete Kubernetes cluster?

No. It does not normally include the scheduler, kubelet, Deployment controller, or real workload networking.

### Why use envtest instead of only a fake client?

Envtest validates CRDs, status subresources, generation, resource versions, defaulting, and API errors more realistically.

### Why call Reconcile directly?

It makes individual test behavior deterministic. The test controls exactly when reconciliation executes.

### How did you test idempotency?

I reconciled a stable PlatformApp twice and verified that the PlatformApp, Deployment, and Service resource versions did not change.

### How did you test Deployment failure?

I created a Deployment with an incompatible immutable selector and verified the controller returned the API error and set Degraded status.

### How did you test Service failure?

I created a Service already controlled by a ConfigMap. The operator could not assign PlatformApp as a second controlling owner, so it returned an error and reported `ServiceReconciliationFailed`.

### Why does readyReplicas remain zero in envtest?

There is no Deployment controller, scheduler, kubelet, or real Pods to update Deployment readiness.

### Is 91.1% coverage enough?

Coverage is a useful indicator, but behavior coverage is more important. The suite covers creation, updates, ownership, idempotency, status, and component-specific failures.

## 41. Chapter summary

The PlatformApp controller tests combine:

- Real Kubernetes API behavior through envtest
- Ginkgo behavior organization
- Gomega assertions
- Direct reconciliation calls
- Success and failure paths
- Resource-version idempotency checks
- k3d integration verification

The reusable testing pattern is:

```text
Create valid custom resource
        ↓
Run reconciliation
        ↓
Retrieve actual resources
        ↓
Assert desired fields and ownership
        ↓
Change desired state
        ↓
Reconcile and assert updates
        ↓
Reconcile unchanged state
        ↓
Assert no API writes
        ↓
Create controlled failures
        ↓
Assert error and Degraded status
```

The next chapter explains how the operator is compiled and packaged into a secure Linux container image and deployed inside Kubernetes.