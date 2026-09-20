# Chapter 2: PlatformApp API and Custom Resource Definition

## Learning objectives

After this chapter, you should be able to explain:

- How Kubernetes APIs are structured
- What a Custom Resource Definition does
- The difference between a CRD and a Custom Resource
- How `PlatformAppSpec` defines desired state
- How `PlatformAppStatus` reports observed state
- Why `Replicas` uses a pointer
- How Kubebuilder markers generate validation and defaults
- Why status uses a subresource
- How Go types become an OpenAPI schema

## 1. Extending the Kubernetes API

Kubernetes includes built-in resource types such as:

- Pod
- Deployment
- Service
- ConfigMap
- Secret

A Custom Resource Definition, or CRD, extends the Kubernetes API with a new resource type.

Our CRD creates:

```text
platformapps.apps.fixnops.com
```

After installing the CRD, Kubernetes understands resources such as:

```yaml
apiVersion: apps.fixnops.com/v1alpha1
kind: PlatformApp
```

The CRD defines the API. An individual object created using that API is called a Custom Resource, or CR.

Example relationship:

| Object | Meaning |
|---|---|
| CRD | Defines the PlatformApp API |
| CR | One PlatformApp instance such as `nginx-demo` |
| Controller | Implements the behavior of PlatformApp |

A CRD without a controller stores and validates data but does not automatically create application resources.

A controller without the CRD cannot read PlatformApp resources because the Kubernetes API server does not know that kind.

## 2. API identity

The PlatformApp API uses:

| Property | Value |
|---|---|
| Group | `apps.fixnops.com` |
| Version | `v1alpha1` |
| Kind | `PlatformApp` |
| Plural | `platformapps` |
| Singular | `platformapp` |
| Scope | Namespaced |

The complete API version is:

```text
apps.fixnops.com/v1alpha1
```

The fully qualified CRD name is:

```text
platformapps.apps.fixnops.com
```

## 3. API version meaning

The version is:

```text
v1alpha1
```

The naming convention communicates maturity:

| Version | General meaning |
|---|---|
| `v1alpha1` | Early API; significant changes may occur |
| `v1beta1` | More stable, but changes may still occur |
| `v1` | Stable API intended for long-term compatibility |

The version name alone does not make an API production-ready. Production readiness also requires compatibility rules, conversion strategy, upgrade testing, documentation, and support expectations.

## 4. Namespaced scope

The CRD declares:

```yaml
scope: Namespaced
```

This means every PlatformApp belongs to a Kubernetes namespace.

Example:

```yaml
metadata:
  name: nginx-demo
  namespace: platformapp-demo
```

Its managed Deployment and Service are created in the same namespace.

Namespaced resources provide:

- Tenant separation
- Namespace-level organization
- Namespace-scoped lifecycle
- Compatibility with namespace policies and quotas
- Simple owner references to namespaced child resources

A cluster-scoped API would not contain a namespace and would normally manage cluster-wide state.

## 5. Creating the API scaffolding

Kubebuilder generated the API and controller scaffolding using an API creation command equivalent to:

```bash
kubebuilder create api \
  --group apps \
  --version v1alpha1 \
  --kind PlatformApp
```

The generated files included:

```text
api/v1alpha1/groupversion_info.go
api/v1alpha1/platformapp_types.go
api/v1alpha1/zz_generated.deepcopy.go
internal/controller/platformapp_controller.go
internal/controller/platformapp_controller_test.go
config/crd/
config/samples/apps_v1alpha1_platformapp.yaml
```

The scaffolding initially contained an example field and an empty reconciliation method. We replaced the example API with the PlatformApp API.

## 6. Desired state: `PlatformAppSpec`

The desired state is defined by:

```go
type PlatformAppSpec struct {
    Image    string `json:"image"`
    Replicas *int32 `json:"replicas,omitempty"`
    Port     int32  `json:"port"`
}
```

The three fields mean:

| Field | Type | Required | Purpose |
|---|---|---|---|
| `image` | string | Yes | Container image for the application |
| `replicas` | pointer to int32 | No | Desired number of application Pods |
| `port` | int32 | Yes | Container and Service port |

The `spec` belongs to the user. The operator reads it but should not treat status information as part of the desired state.

## 7. JSON tags

Each API field has a JSON tag:

```go
Image string `json:"image"`
```

Kubernetes resources are serialized as JSON internally, even when users submit YAML.

The tag maps the Go field:

```text
Image
```

to the Kubernetes API field:

```text
image
```

For replicas:

```go
Replicas *int32 `json:"replicas,omitempty"`
```

`omitempty` means the field can be omitted from serialized data when it has no value.

API fields must have correct JSON tags so that Kubernetes clients and the API server can serialize them.

## 8. Why replicas uses a pointer

Replicas is declared as:

```go
Replicas *int32
```

A pointer allows the program to distinguish between:

- The user omitted replicas: `nil`
- The user explicitly supplied a replica value
- Kubernetes defaulted the value to one

If replicas were a plain `int32`, its zero value would be `0`. The controller could not easily distinguish an omitted value from an explicitly supplied zero before validation and defaulting.

The controller also protects itself with a runtime default:

```go
desiredReplicas := int32(1)
if platformApp.Spec.Replicas != nil {
    desiredReplicas = *platformApp.Spec.Replicas
}
```

The API server default and controller fallback provide defense in depth.

## 9. Validation markers

Kubebuilder markers are specially formatted Go comments read by `controller-gen`.

### Image validation

```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
Image string `json:"image"`
```

This generates:

```yaml
image:
  minLength: 1
  type: string
```

It also places `image` in the required-field list.

The API server rejects an empty image.

### Replica validation and default

```go
// +kubebuilder:default:=1
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=10
// +optional
Replicas *int32 `json:"replicas,omitempty"`
```

This generates:

```yaml
replicas:
  default: 1
  minimum: 1
  maximum: 10
  format: int32
  type: integer
```

The API server:

- Defaults omitted replicas to one
- Rejects values below one
- Rejects values above ten

### Port validation

```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=65535
Port int32 `json:"port"`
```

This limits the port to the valid TCP/UDP port range.

## 10. Why API-server validation matters

Validation should happen before reconciliation whenever possible.

Without CRD validation:

1. Kubernetes accepts invalid data.
2. The controller receives it.
3. Controller code must detect every invalid case.
4. Invalid resources remain stored in the API.
5. Users receive failures later and with less clarity.

With CRD validation:

1. Kubernetes rejects invalid data during admission.
2. The invalid object is not stored.
3. The user receives an immediate field-specific error.
4. The controller only processes structurally valid objects.

We tested a PlatformApp missing `spec.port`:

```bash
kubectl apply --dry-run=server -f - <<'EOF'
apiVersion: apps.fixnops.com/v1alpha1
kind: PlatformApp
metadata:
  name: missing-port
  namespace: platformapp-demo
spec:
  image: nginx:1.27
EOF
```

The API server rejected it:

```text
spec.port: Required value
```

That validation occurred before the controller was involved.

## 11. Observed state: `PlatformAppStatus`

Status is defined as:

```go
type PlatformAppStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
}
```

The fields mean:

| Field | Purpose |
|---|---|
| `observedGeneration` | PlatformApp generation processed by the controller |
| `readyReplicas` | Number of application replicas currently ready |
| `conditions` | Detailed state and reason information |

Status belongs to the controller. Users normally modify `spec`, while the controller updates `status`.

## 12. Generation and observed generation

Kubernetes increments:

```text
metadata.generation
```

when the desired state changes.

The controller records:

```text
status.observedGeneration
```

after processing that generation.

Example:

```text
metadata.generation: 4
status.observedGeneration: 4
```

This means the controller has processed the current desired state.

If generation is 5 but observedGeneration is 4, the status may be stale because the controller has not yet processed generation 5.

## 13. Conditions

Conditions use the Kubernetes standard type:

```go
metav1.Condition
```

Each condition contains:

- `type`
- `status`
- `reason`
- `message`
- `observedGeneration`
- `lastTransitionTime`

Our condition types are:

```text
Available
Progressing
Degraded
```

Using standard conditions makes status easier for:

- Users
- Automation
- Monitoring systems
- Command-line tools
- Other controllers

## 14. Conditions as a map-style list

The markers:

```go
// +listType=map
// +listMapKey=type
Conditions []metav1.Condition `json:"conditions,omitempty"`
```

tell Kubernetes that each condition is uniquely identified by its `type`.

This prevents several separate `Available` entries from being treated as unrelated list items.

Conceptually, the list behaves like a map keyed by:

```text
condition.type
```

## 15. Status subresource

The PlatformApp type includes:

```go
// +kubebuilder:subresource:status
```

This causes the CRD to include:

```yaml
subresources:
  status: {}
```

The API server then exposes a separate status endpoint:

```text
/status
```

Benefits include:

- Status updates do not overwrite spec changes
- Separate RBAC can protect status
- Normal user updates do not accidentally replace status
- Status changes do not increment `metadata.generation`

The controller has separate permissions for:

```text
platformapps/status
```

## 16. Root object markers

The PlatformApp type uses:

```go
// +kubebuilder:object:root=true
```

This marks it as a top-level Kubernetes API object.

PlatformAppList also uses the root marker because list operations return a Kubernetes list object containing multiple PlatformApps.

## 17. Kubernetes metadata embedding

PlatformApp embeds:

```go
metav1.TypeMeta
metav1.ObjectMeta
```

`TypeMeta` represents fields such as:

```yaml
apiVersion: apps.fixnops.com/v1alpha1
kind: PlatformApp
```

`ObjectMeta` represents fields such as:

```yaml
metadata:
  name: nginx-demo
  namespace: platformapp-demo
  labels: {}
  annotations: {}
  generation: 4
  resourceVersion: "31242"
  uid: ...
```

Embedding makes these fields part of the PlatformApp Go object.

## 18. Scheme registration

The API types are registered with the runtime scheme:

```go
func init() {
    SchemeBuilder.Register(func(s *runtime.Scheme) error {
        s.AddKnownTypes(
            SchemeGroupVersion,
            &PlatformApp{},
            &PlatformAppList{},
        )
        return nil
    })
}
```

The scheme connects:

- Go types
- API groups
- API versions
- Kubernetes kinds

Without scheme registration, controller-runtime cannot correctly encode, decode, or recognize PlatformApp objects.

## 19. Deep-copy generation

Kubernetes API machinery frequently copies objects.

The generated file:

```text
api/v1alpha1/zz_generated.deepcopy.go
```

contains methods such as:

```go
func (in *PlatformAppSpec) DeepCopyInto(out *PlatformAppSpec)
```

A deep copy creates independent copies of pointer, slice, map, and nested object data.

For example, `Replicas` is a pointer, so its referenced value must be copied rather than sharing the same memory address.

Do not normally edit `zz_generated.deepcopy.go` manually.

Regenerate it using:

```bash
make generate
```

## 20. CRD generation

After modifying API types or markers, run:

```bash
make manifests
```

This executes `controller-gen` and regenerates:

```text
config/crd/bases/apps.fixnops.com_platformapps.yaml
```

The generated CRD contains:

- Group and version
- Kind and plural names
- Namespaced scope
- OpenAPI validation schema
- Default values
- Required fields
- Status subresource

Run:

```bash
make generate
```

to regenerate Go deep-copy code.

The usual workflow after changing API types is:

```bash
make manifests
make generate
make fmt
make test
```

## 21. Installing and discovering the CRD

Install the CRD:

```bash
make install
```

Verify discovery:

```bash
kubectl api-resources \
  --api-group=apps.fixnops.com
```

Expected resource:

```text
platformapps   apps.fixnops.com/v1alpha1   true   PlatformApp
```

Inspect the CRD:

```bash
kubectl get crd platformapps.apps.fixnops.com \
  -o yaml
```

The CRD is cluster-scoped because it defines an API for the whole cluster. Individual PlatformApp resources are namespaced.

## 22. Sample resource and defaulting

The sample intentionally omits replicas:

```yaml
apiVersion: apps.fixnops.com/v1alpha1
kind: PlatformApp
metadata:
  name: nginx-demo
  namespace: platformapp-demo
spec:
  image: nginx:1.27
  port: 80
```

After creation, the API server stores:

```yaml
spec:
  image: nginx:1.27
  port: 80
  replicas: 1
```

This proves that CRD defaulting is working.

Apply it using:

```bash
kubectl apply \
  -f config/samples/apps_v1alpha1_platformapp.yaml
```

Inspect it:

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o yaml
```

## 23. CRD versus admission webhook validation

CRD/OpenAPI validation is best for rules such as:

- Required fields
- Minimum and maximum numbers
- String length
- Enumerated values
- Simple field patterns

Admission webhooks are useful for logic such as:

- Cross-field validation
- Cluster-aware validation
- Conditional rules
- Complex defaulting
- External policy checks

Our current image, replicas, and port rules are simple and belong in the CRD schema.

A later chapter will cover webhook use cases.

## 24. Common mistakes

### Making optional numeric fields non-pointers

A non-pointer numeric field has zero as its default Go value. This can make omission difficult to distinguish from an explicitly supplied zero.

### Forgetting JSON tags

The field may not serialize as intended through the Kubernetes API.

### Editing generated CRD YAML

The next `make manifests` may overwrite the manual changes.

### Updating status through the main resource endpoint

Use the status client operation and status subresource instead.

### Treating `v1alpha1` as automatically unstable code

The label describes the API compatibility promise, not necessarily the quality of the controller implementation.

### Assuming CRD defaulting replaces controller safeguards

Controllers should still handle missing or unexpected data defensively, particularly during upgrades and tests.

## 25. Interview questions and answers

### What is the difference between a CRD and a CR?

A CRD defines a new Kubernetes API type. A CR is one object created using that API.

### Why did you make PlatformApp namespaced?

The managed application resources are namespaced. Namespaced scope provides isolation and allows the PlatformApp, Deployment, and Service to share the same namespace and owner-reference lifecycle.

### Why is replicas a pointer?

It distinguishes an omitted field from an explicitly supplied numeric value and works correctly with optional fields and API defaulting.

### Where does validation happen?

Structural validation and defaulting happen in the Kubernetes API server using the OpenAPI schema generated from Kubebuilder markers.

### Why use a status subresource?

It separates user-managed desired state from controller-managed observed state and enables independent authorization and update behavior.

### What is observedGeneration?

It records the resource generation processed by the controller. Comparing it with `metadata.generation` shows whether status reflects the latest desired state.

### Why use `metav1.Condition`?

It follows Kubernetes API conventions and provides structured state, reason, message, generation, and transition-time information.

### What does `make manifests` do?

It runs controller generation to regenerate CRDs, RBAC, and webhook manifests from Go types and Kubebuilder markers.

### What does `make generate` do?

It regenerates Go code such as deep-copy functions required by Kubernetes API machinery.

## 26. Practical verification checklist

```bash
make manifests
make generate
make fmt
make test
```

```bash
kubectl api-resources \
  --api-group=apps.fixnops.com
```

```bash
kubectl get crd platformapps.apps.fixnops.com
```

```bash
kubectl get platformapp nginx-demo \
  -n platformapp-demo \
  -o yaml
```

Confirm:

- CRD is established
- Resource is namespaced
- Image and port are required
- Replicas defaults to one
- Replicas range is 1–10
- Port range is 1–65535
- Status subresource exists
- Conditions use map-list semantics

## 27. Chapter summary

The PlatformApp API establishes a contract between users and the operator.

The user owns:

```text
spec
```

The controller owns:

```text
status
```

Kubebuilder markers convert Go comments and types into a Kubernetes OpenAPI schema. The API server performs defaulting and validation before reconciliation begins.

The next chapter explains how the controller reads this API and reconciles Deployments and Services.