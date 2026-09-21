# Go for Kubernetes Operator Development

This learning guide teaches Go from absolute programming fundamentals through the concepts required to understand, build, test, and explain a Kubernetes Operator.

The guide assumes that the learner:

- Has Kubernetes or OpenShift operational knowledge.
- Has little or no previous programming experience.
- Wants to learn Go specifically for Kubernetes Operator development.
- Wants practical experience rather than theory alone.
- Needs to explain the implementation confidently during interviews.

The practical reference project is the `PlatformApp` Operator.

## Reference Project

| Item | Value |
|---|---|
| Repository | `github.com/fixnops/platformapp-operator` |
| API group | `apps.fixnops.com` |
| API version | `v1alpha1` |
| Kind | `PlatformApp` |
| Development cluster | `k3d-operator-lab` |
| Managed resources | `Deployment` and `Service` |

Example Custom Resource:

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

The Operator reads this resource and maintains the following desired state:

```text
PlatformApp
├── Deployment
│   ├── Image: nginx:1.27
│   └── Replicas: 2
└── ClusterIP Service
    └── Port: 80
```

## Learning Objective

After completing this guide, the learner should be able to:

1. Read and understand fundamental Go code.
2. Write small Go programs independently.
3. Understand Go types, functions, structs, methods, and pointers.
4. Work with slices, maps, loops, and conditions.
5. Handle errors using standard Go patterns.
6. Understand interfaces and composition.
7. Explain packages, modules, and dependencies.
8. Understand JSON tags and Kubernetes API types.
9. Explain `context.Context`, goroutines, and channels.
10. Read and modify Kubernetes controller code.
11. Explain the complete reconciliation process.
12. Write and understand basic Go tests.
13. Explain the `PlatformApp` Operator during an interview.

This guide does not attempt to cover every feature of Go. It focuses on the language concepts required for Kubernetes APIs, controllers, automation, and Operator development.

## Teaching Method

Every chapter follows the same learning sequence:

```text
Plain-language theory
        ↓
Real-life analogy
        ↓
Small Go example
        ↓
Run and inspect the result
        ↓
Connect it to PlatformApp
        ↓
Complete an exercise
        ↓
Answer interview questions
```

Each chapter contains:

- A plain-language explanation
- The reason the concept exists
- Syntax explained line by line
- Small executable examples
- Expected output
- Operator-specific examples
- Common mistakes
- Practical exercises
- Interview questions
- A completion checkpoint

## Learning Path

### Foundation Stage

1. [Programming and Go Foundations](01-programming-and-go-foundations.md)
2. [Go Program Structure, Packages, and Modules](02-go-program-structure-packages-and-modules.md)
3. [Variables, Constants, Types, and Zero Values](03-variables-constants-types-and-zero-values.md)
4. [Functions, Conditions, Loops, and Scope](04-functions-conditions-loops-and-scope.md)

### Core Go Stage

5. [Structs, Methods, and Pointers](05-structs-methods-and-pointers.md)
6. [Arrays, Slices, Maps, and Range](06-arrays-slices-maps-and-range.md)
7. [Errors, Defer, Panic, and Recovery](07-errors-defer-panic-and-recovery.md)
8. [Interfaces, Composition, and Type Assertions](08-interfaces-composition-and-type-assertions.md)
9. [JSON Tags, API Types, and Data Conversion](09-json-tags-api-types-and-data-conversion.md)
10. [Context, Goroutines, and Channels](10-context-goroutines-and-channels.md)

### Operator Development Stage

11. [Testing and Code Quality](11-testing-and-code-quality.md)
12. [Kubernetes Client and Controller Runtime](12-kubernetes-client-and-controller-runtime.md)
13. [Reconcile Loop Line by Line](13-reconcile-loop-line-by-line.md)
14. [Building an Operator with Go](14-building-an-operator-with-go.md)

### Interview and Practice Stage

15. [Go and Operator Interview Guide](15-go-and-operator-interview-guide.md)
16. [Exercises and Final Checkpoints](16-exercises-and-final-checkpoints.md)

## How Go Fits into an Operator

The complete execution flow is:

```text
Go source code
      ↓
Go compiler
      ↓
Operator executable
      ↓
Container image
      ↓
Operator Deployment
      ↓
Operator pod
      ↓
Controller watches Kubernetes resources
      ↓
Reconcile method compares desired and actual state
      ↓
Controller creates, updates, or deletes resources
```

Operator SDK generates the initial project structure, but the developer must understand and implement the API and reconciliation behaviour.

```text
Operator SDK     = Project scaffolding and development tools
Go               = Programming language
controller-runtime = Controller libraries
CRD              = New API definition
Custom Resource  = Desired state submitted by a user
Controller       = Watches relevant resources
Reconcile        = Compares and corrects state
```

## Important Project Files

The following files demonstrate how Go is used in the `PlatformApp` Operator:

```text
platformapp-operator/
├── api/v1alpha1/
│   ├── platformapp_types.go
│   ├── groupversion_info.go
│   └── zz_generated.deepcopy.go
├── internal/controller/
│   └── platformapp_controller.go
├── cmd/
│   └── main.go
├── config/
├── go.mod
├── go.sum
└── Makefile
```

Their responsibilities are:

| File | Responsibility |
|---|---|
| `platformapp_types.go` | Defines `PlatformAppSpec`, `PlatformAppStatus`, and API markers |
| `groupversion_info.go` | Registers the API group and version |
| `zz_generated.deepcopy.go` | Contains generated deep-copy implementations |
| `platformapp_controller.go` | Contains reconciliation logic |
| `cmd/main.go` | Creates and starts the controller manager |
| `go.mod` | Declares the module and its dependencies |
| `go.sum` | Records dependency checksums |
| `Makefile` | Provides development and build commands |

Generated files should normally be regenerated through project commands rather than edited manually.

Common commands include:

```bash
make generate
make manifests
make fmt
make test
make build
make install
make run
```

## Learning Rules

Follow these rules throughout the guide:

1. Type and run every example.
2. Do not memorize code without understanding it.
3. Read compiler errors instead of immediately deleting the code.
4. Change example values and observe the result.
5. Complete exercises before reviewing solutions.
6. Connect every Go concept to the Operator project.
7. Explain completed concepts in your own words.
8. Commit documentation and exercises after verifying them.
9. Do not manually edit generated Kubernetes files.
10. Prefer simple, readable code over clever code.

## Practical Exercise Convention

Exercises use separate directories so they do not interfere with the Operator source code.

Example:

```text
go-learning/
├── chapter-01/
├── chapter-02/
├── chapter-03/
└── ...
```

Each directory may have its own small Go program:

```text
chapter-01/
├── go.mod
└── main.go
```

A program is normally formatted and executed with:

```bash
go fmt ./...
go run .
```

Tests are executed with:

```bash
go test ./...
```

## Progress Tracker

Mark a chapter complete only after finishing its practical exercises and interview checkpoint.

- [ ] Chapter 1: Programming and Go Foundations
- [ ] Chapter 2: Program Structure, Packages, and Modules
- [ ] Chapter 3: Variables, Constants, Types, and Zero Values
- [ ] Chapter 4: Functions, Conditions, Loops, and Scope
- [ ] Chapter 5: Structs, Methods, and Pointers
- [ ] Chapter 6: Arrays, Slices, Maps, and Range
- [ ] Chapter 7: Errors, Defer, Panic, and Recovery
- [ ] Chapter 8: Interfaces, Composition, and Type Assertions
- [ ] Chapter 9: JSON Tags, API Types, and Data Conversion
- [ ] Chapter 10: Context, Goroutines, and Channels
- [ ] Chapter 11: Testing and Code Quality
- [ ] Chapter 12: Kubernetes Client and Controller Runtime
- [ ] Chapter 13: Reconcile Loop Line by Line
- [ ] Chapter 14: Building an Operator with Go
- [ ] Chapter 15: Go and Operator Interview Guide
- [ ] Chapter 16: Exercises and Final Checkpoints

## Final Success Criteria

The learning path is complete when the learner can:

- Explain the `PlatformApp` API.
- Read `platformapp_controller.go`.
- Explain every part of the `Reconcile()` signature.
- Describe the desired-state reconciliation process.
- Modify a field or validation rule safely.
- Add basic controller behaviour.
- Handle errors correctly.
- Run and understand tests.
- Explain how the Operator performs self-healing.
- Present the project confidently in a technical interview.

The target is not to memorize the entire Go language.

The target is to understand enough Go to build, troubleshoot, improve, and explain a Kubernetes Operator.