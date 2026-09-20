# Chapter 6: Container Image and In-Cluster Deployment

## Learning objectives

After this chapter, you should be able to explain:

- How the Go manager becomes a container image
- Why the Dockerfile uses multiple stages
- Why CGO is disabled
- How image architecture must match Kubernetes nodes
- Why the runtime image is distroless
- How the operator runs as a non-root user
- Why Docker and k3d use different image stores
- How Kustomize renders the installation
- How the operator runs inside Kubernetes
- How to verify an operator Deployment

## 1. Development binary versus production container

During early development, the manager ran directly on the Mac:

```bash
make build
```

This produced:

```text
bin/manager
```

Because it was built on macOS ARM64, the binary was:

```text
Mach-O ARM64
```

A Kubernetes node runs Linux, so that Mac binary cannot be copied directly into a Kubernetes container.

The production-style workflow is:

```text
Go source
    ↓
Linux manager binary
    ↓
Container image
    ↓
Kubernetes Deployment
    ↓
Operator Pod
```

## 2. Dockerfile stages

The Dockerfile uses a multi-stage build.

The first stage compiles the manager:

```dockerfile
ARG BASE_IMAGE=golang:1.26
FROM ${BASE_IMAGE} AS builder
```

The second stage packages only the compiled binary:

```dockerfile
FROM gcr.io/distroless/static:nonroot
```

The large compiler environment is not included in the final runtime image.

## 3. Builder stage

The builder declares platform arguments:

```dockerfile
ARG TARGETOS
ARG TARGETARCH
```

These allow Docker BuildKit to supply target values such as:

```text
TARGETOS=linux
TARGETARCH=arm64
```

The working directory is:

```dockerfile
WORKDIR /workspace
```

All compilation steps occur under this directory.

## 4. Dependency caching

The Dockerfile copies module metadata before application source:

```dockerfile
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
```

Docker caches image layers.

If application source changes but `go.mod` and `go.sum` remain unchanged, Docker can reuse the dependency-download layer.

Then the remaining source is copied:

```dockerfile
COPY . .
```

The `.dockerignore` file prevents unnecessary files from entering the build context.

## 5. Building the manager

The compile command is:

```dockerfile
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    go build -a -o manager cmd/main.go
```

### `CGO_ENABLED=0`

Disables dependencies on the C runtime.

This helps produce a self-contained Go binary suitable for a minimal static runtime image.

### `GOOS`

Defines the target operating system.

The default is:

```text
linux
```

### `GOARCH`

Defines the target CPU architecture.

For the current environment:

```text
arm64
```

### `-a`

Forces rebuilding of packages during compilation.

### `-o manager`

Writes the output binary as:

```text
/workspace/manager
```

## 6. Runtime stage

The final stage uses:

```dockerfile
FROM gcr.io/distroless/static:nonroot
```

Distroless images contain only the minimal files required to run the application.

They generally do not include:

- Shell
- Package manager
- Compiler
- Debugging tools
- General command-line utilities

Benefits include:

- Smaller attack surface
- Fewer packages to patch
- Smaller image content
- Reduced opportunity for interactive modification

The tradeoff is that troubleshooting cannot rely on opening a shell inside the container.

## 7. Copying the binary

The manager binary is copied from the builder stage:

```dockerfile
COPY --from=builder /workspace/manager .
```

Only the compiled binary enters the final stage.

The source code, module cache, Go compiler, and build tools remain in the discarded builder stage.

## 8. Non-root runtime

The Dockerfile specifies:

```dockerfile
USER 65532:65532
```

The manager runs using non-root UID and GID 65532.

The entry point is:

```dockerfile
ENTRYPOINT ["/manager"]
```

When Kubernetes starts the container, it executes the manager directly.

## 9. Building the local image

The Makefile target is:

```make
docker-build:
	$(CONTAINER_TOOL) build -t ${IMG} .
```

The local image was built using:

```bash
make docker-build \
  IMG=platformapp-operator:v0.1.0-local
```

The `-local` suffix communicates that this is not a published production release.

## 10. Platform compatibility

The development environment was:

| Component | Platform |
|---|---|
| Docker client | Darwin ARM64 |
| Docker server | Linux ARM64 |
| k3d server node | Linux ARM64 |
| k3d agent node | Linux ARM64 |
| Operator image | Linux ARM64 |

The image was verified using:

```bash
docker image inspect \
  platformapp-operator:v0.1.0-local \
  --format='os={{.Os}} architecture={{.Architecture}} user={{.Config.User}} entrypoint={{json .Config.Entrypoint}}'
```

The result confirmed:

```text
os=linux
architecture=arm64
user=65532:65532
entrypoint=["/manager"]
```

## 11. Testing the image entry point

The manager was tested using:

```bash
docker run --rm \
  platformapp-operator:v0.1.0-local \
  --help
```

This proved:

- The container started
- `/manager` existed
- The binary architecture was correct
- The non-root user could execute it
- The entry point passed command-line arguments correctly

`--help` exits before the manager tries to connect to Kubernetes.

## 12. Why shell commands do not work

Commands such as this are not expected to work:

```bash
docker run --rm \
  platformapp-operator:v0.1.0-local \
  sh
```

The distroless image does not contain `sh`.

This is intentional.

Debugging options include:

- Controller logs
- Metrics
- Health endpoints
- Kubernetes Events
- Ephemeral debug containers
- A separate debug image
- Local execution of the manager

## 13. Docker image size

Docker reported approximately:

```text
31.6 MB content size
```

Docker Desktop also displayed a larger disk-usage value because local accounting may include:

- Unpacked layers
- Shared layers
- Build metadata
- Attestations

The content size better represents the image payload.

## 14. Docker and k3d image stores

Building an image in Docker Desktop does not automatically make it available to k3d nodes.

Docker Desktop stores the image in Docker's image store.

Each k3d node runs K3s with its own containerd image store.

The image was transferred using:

```bash
k3d image import \
  platformapp-operator:v0.1.0-local \
  --cluster operator-lab
```

This imported the image into both:

- k3d server node
- k3d agent node

## 15. Verifying the node image stores

The imported image was verified using:

```bash
docker exec k3d-operator-lab-server-0 \
  crictl images | grep platformapp-operator
```

and:

```bash
docker exec k3d-operator-lab-agent-0 \
  crictl images | grep platformapp-operator
```

Both nodes reported the same image configuration ID and content size.

This allows Kubernetes to start the image without downloading it from an external registry.

## 16. Image pull behavior

The local tag is not `latest`:

```text
v0.1.0-local
```

When `imagePullPolicy` is omitted for a non-latest tag, Kubernetes normally defaults to:

```text
IfNotPresent
```

Because the image already exists on both k3d nodes, the kubelet uses the imported image.

The Pod event confirmed:

```text
Container image already present on machine
```

A real production cluster should pull from an accessible registry rather than rely on manual node imports.

## 17. Kustomize structure

The top-level configuration is:

```text
config/default/kustomization.yaml
```

It combines:

```yaml
resources:
- ../crd
- ../rbac
- ../manager
- metrics_service.yaml
```

It also sets:

```yaml
namespace: platformapp-operator-system
namePrefix: platformapp-operator-
```

Kustomize transforms generic generated names into project-specific names.

Examples:

| Source name | Rendered name |
|---|---|
| `controller-manager` | `platformapp-operator-controller-manager` |
| `manager-role` | `platformapp-operator-manager-role` |
| `system` namespace | `platformapp-operator-system` |

Kustomize also rewrites references between renamed resources.

## 18. Manager manifest

The manager manifest defines:

- Namespace
- Operator Deployment
- Replicas
- Pod labels
- ServiceAccount
- Manager command
- Leader-election argument
- Health port
- Probes
- Resources
- Security context
- Affinity

The generic source image is:

```text
controller:latest
```

Kustomize replaces it when `IMG` is supplied.

## 19. Deploy target

The deployment command was:

```bash
make deploy \
  IMG=platformapp-operator:v0.1.0-local
```

The Makefile:

1. Regenerated manifests.
2. Updated the Kustomize image override.
3. Rendered `config/default`.
4. Piped the result to `kubectl apply`.

The installation created or updated:

- Namespace
- CRD
- ServiceAccount
- Roles
- ClusterRoles
- RoleBindings
- ClusterRoleBindings
- Metrics Service
- Operator Deployment
- PodDisruptionBudget

## 20. Operator namespace

The operator runs in:

```text
platformapp-operator-system
```

The managed PlatformApp can exist in another namespace:

```text
platformapp-demo
```

The ClusterRole allows the operator to watch PlatformApps and managed resources across namespaces.

The manager itself remains isolated in its system namespace.

## 21. In-cluster authentication

Inside Kubernetes, the manager does not use the Mac kubeconfig.

The Pod uses:

```text
platformapp-operator-controller-manager
```

as its ServiceAccount.

Kubernetes mounts a projected ServiceAccount token and cluster CA information into the Pod.

controller-runtime uses in-cluster configuration to connect to the Kubernetes API.

## 22. Deployment arguments

The running container uses arguments including:

```text
--metrics-bind-address=:8443
--leader-elect
--health-probe-bind-address=:8081
```

These enable:

- Secure metrics
- Leader election
- Health and readiness endpoints

The same manager binary supports local and in-cluster execution. Runtime flags change its behavior.

## 23. Verifying rollout

The operator rollout was checked using:

```bash
kubectl rollout status \
  deployment/platformapp-operator-controller-manager \
  -n platformapp-operator-system \
  --timeout=120s
```

A successful result means the requested Deployment replicas became available.

## 24. Inspecting the running operator

Useful commands include:

```bash
kubectl get deployment,pods,serviceaccount,service \
  -n platformapp-operator-system
```

```bash
kubectl get pods \
  -n platformapp-operator-system \
  -l control-plane=controller-manager \
  -o wide
```

```bash
kubectl logs \
  -n platformapp-operator-system \
  deployment/platformapp-operator-controller-manager
```

The logs confirmed:

- Manager startup
- Health server startup
- Metrics server startup
- Leader Lease acquisition
- Watch registration
- Controller worker startup
- Existing PlatformApp reconciliation

## 25. Proving in-cluster reconciliation

The existing PlatformApp was updated:

```bash
kubectl patch platformapp nginx-demo \
  -n platformapp-demo \
  --type=merge \
  -p '{"spec":{"replicas":2}}'
```

The in-cluster operator logs showed:

```text
image=nginx:1.27
replicas=2
generation=4
```

The Deployment reached:

```text
2/2 ready
```

The PlatformApp reported:

```text
generation=4
observedGeneration=4
readyReplicas=2
```

This proved the reconciliation came from the running Kubernetes Pod rather than the local Mac binary.

## 26. Avoiding two independent controllers

Before starting the in-cluster operator, the local manager process was checked:

```bash
pgrep -fl \
  'operator-build/bin/manager|/manager'
```

No local manager process was running.

This is important because the local manager had previously been started without the same in-cluster leader-election setup.

Two independent controllers could otherwise process the same resources.

## 27. Rendering an installer

Kubebuilder provides:

```bash
make build-installer IMG=<image>
```

This:

1. Sets the Kustomize image.
2. Renders all installation resources.
3. Writes:

```text
dist/install.yaml
```

The installer can provide a single-file installation artifact.

During local testing, it was rendered using:

```text
platformapp-operator:v0.1.0-local
```

The local image override and generated `dist/` directory were removed before committing because they were environment-specific.

A future release workflow will generate an installer using a published immutable image.

## 28. Why local image names are not production releases

This image reference:

```text
platformapp-operator:v0.1.0-local
```

has no registry hostname.

It works only because it was manually imported into the k3d nodes.

A production image reference should include a registry, repository, and immutable version, for example:

```text
<registry>/<owner>/platformapp-operator:<version>
```

The exact production registry must be selected and configured before publishing.

## 29. Multi-architecture builds

The Makefile supports Buildx with platforms such as:

```text
linux/arm64
linux/amd64
linux/s390x
linux/ppc64le
```

A multi-architecture image uses a manifest list.

When a node pulls the image, the registry supplies the correct platform-specific manifest.

Local development built only Linux ARM64 because both the Docker server and k3d nodes were ARM64.

Production releases should normally support at least:

```text
linux/amd64
linux/arm64
```

The final platform list should match supported production environments.

## 30. Immutable image references

Mutable tags such as:

```text
latest
```

can point to different content over time.

Production releases should use version tags such as:

```text
v0.1.0
```

For maximum reproducibility, deployment systems can pin an image digest:

```text
image@sha256:<digest>
```

A digest identifies exact image content.

## 31. Source changes versus build artifacts

Building and importing an image should not change tracked source files.

The repository remained clean after:

```bash
make docker-build
k3d image import
```

Commands such as `make deploy IMG=...` or `make build-installer IMG=...` can modify the Kustomize image override.

Environment-specific image overrides should be reviewed before committing.

## 32. Common mistakes

### Building a macOS binary for Linux

The binary operating system must match the container runtime.

### Building AMD64 for ARM64 nodes

The architecture must match unless emulation is intentionally used.

### Assuming Docker images automatically exist in k3d

The node containerd stores are separate from the Docker image store.

### Using a local image name in production manifests

Other clusters cannot pull an image that exists only on a developer machine.

### Expecting a shell in distroless

Use logs, metrics, probes, and debug containers instead.

### Running as root unnecessarily

The manager does not require root privileges.

### Committing local Kustomize image overrides

Release automation should set the correct published image.

### Using only `latest`

Mutable tags make rollbacks and incident investigation harder.

## 33. Interview questions and answers

### Why use a multi-stage Dockerfile?

The builder stage contains the compiler and dependencies. The final stage contains only the manager binary and minimal runtime files, reducing image size and attack surface.

### Why disable CGO?

It produces a self-contained Go binary without a dependency on the target system's C runtime, which is suitable for a static distroless image.

### Why use distroless?

It minimizes packages and tools in the runtime image, reducing the attack surface and patching burden.

### How does the Pod authenticate to Kubernetes?

It uses its mounted ServiceAccount token and the cluster CA through controller-runtime's in-cluster configuration.

### Why was the image imported into k3d?

Docker Desktop and k3d node containerd use separate image stores. Importing makes the locally built image available to the nodes.

### How did you verify image compatibility?

I inspected the image OS, architecture, user, and entry point and confirmed they matched the Linux ARM64 k3d nodes.

### What is the difference between `make build` and `make docker-build`?

`make build` produces a local binary for the development machine. `make docker-build` creates a container image containing a Linux manager binary.

### What does Kustomize do?

It composes resources, applies namespaces and name prefixes, patches Deployments, substitutes image references, and rewrites resource references.

### How did you prove the operator ran inside Kubernetes?

I inspected the running Pod and ServiceAccount, checked manager logs, patched the PlatformApp, and verified the Deployment and PlatformApp status were updated by the in-cluster controller.

## 34. Repeatable packaging checklist

For a future operator:

1. Confirm the Go module version.
2. Use a multi-stage Dockerfile.
3. Disable CGO when compatible.
4. Build for Linux.
5. Select supported CPU architectures.
6. Use a minimal non-root runtime image.
7. Inspect image metadata.
8. Test the entry point.
9. Push to an accessible registry or import into a local cluster.
10. Render manifests with the intended image.
11. Deploy using a dedicated namespace and ServiceAccount.
12. Verify rollout and logs.
13. Test reconciliation from the running Pod.
14. Avoid committing local-only image overrides.

## 35. Chapter summary

The manager moved through four forms:

```text
Go source
    ↓
Local macOS binary
    ↓
Linux ARM64 container image
    ↓
Highly available Kubernetes Deployment
```

The final container uses a minimal distroless image, runs as a non-root user, and starts `/manager` directly.

Kustomize combines CRDs, RBAC, the manager Deployment, metrics, and availability resources into an installable operator.

The next chapter explains ServiceAccounts, least-privilege RBAC, security contexts, health probes, and authenticated metrics.