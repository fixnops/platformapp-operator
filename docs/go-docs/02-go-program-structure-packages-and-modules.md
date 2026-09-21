# Chapter 2: Go Program Structure, Packages, and Modules

## Chapter Goal

This chapter explains how Go source code is organized.

After completing it, you should understand:

- How a Go program starts
- What packages are
- Why packages exist
- How imports work
- Exported and unexported identifiers
- What a Go module is
- The purpose of `go.mod`
- The purpose of `go.sum`
- How external dependencies are managed
- How these concepts appear in the `PlatformApp` Operator

This chapter focuses on program organization. Variables, functions, and types will be explored in greater detail in later chapters.

---

# 1. Why Program Structure Matters

A tiny program can exist in one file:

```text
main.go
```

A real application usually contains many responsibilities:

- Starting the program
- Reading configuration
- Defining data types
- Calling APIs
- Processing business logic
- Handling errors
- Writing logs
- Running tests

Keeping everything in one large file makes the code difficult to:

- Understand
- Test
- Reuse
- Review
- Maintain
- Troubleshoot

Go organizes code using:

```text
Modules
└── Packages
    └── Files
        └── Functions, types, and variables
```

A simple memory map is:

```text
Module  = Complete Go project
Package = Related Go code within the project
File    = One source-code document
Function = Reusable behaviour
Type    = Description of data
```

---

# 2. Go Source Files

Go source files use the `.go` extension.

Examples:

```text
main.go
platformapp_types.go
platformapp_controller.go
groupversion_info.go
```

A Go source file normally contains:

```go
package packagename

import (
	"some/package"
)

type Example struct {
	Name string
}

func exampleFunction() {
}
```

The usual order is:

```text
1. Package declaration
2. Imports
3. Constants and variables
4. Type definitions
5. Functions and methods
```

Not every file must contain every section.

---

# 3. Package Declaration

Every non-empty Go source file begins with a package declaration:

```go
package main
```

or:

```go
package controller
```

or:

```go
package v1alpha1
```

A package groups related Go files and identifiers.

Files in the same directory normally belong to the same package.

Example:

```text
calculator/
├── add.go
└── subtract.go
```

Both files might begin with:

```go
package calculator
```

Their functions belong to the same package and can work together.

---

# 4. Package as a Team

Think of a package as a specialized team.

For example:

```text
controller package
    Responsible for reconciliation

v1alpha1 package
    Responsible for API definitions

main package
    Responsible for starting the application
```

A team should have a clear purpose.

Similarly, a package should group closely related responsibilities.

A package should not become a random collection of unrelated code.

---

# 5. Package Name and Directory Name

The package name commonly matches its directory name.

Example:

```text
internal/controller/
└── platformapp_controller.go
```

Inside the file:

```go
package controller
```

Example:

```text
api/v1alpha1/
├── platformapp_types.go
└── groupversion_info.go
```

Inside these files:

```go
package v1alpha1
```

Matching package and directory names makes the code easier to understand.

There are special cases where they differ, but beginners should follow the conventional pattern unless there is a clear reason not to.

---

# 6. The `main` Package

`main` is a special package.

A program that produces an executable must contain:

```go
package main
```

and:

```go
func main() {
}
```

The program starts execution from `main()`.

Example:

```go
package main

import "fmt"

func main() {
	fmt.Println("Starting program")
}
```

The execution flow is:

```text
Operating system starts binary
             ↓
Go runtime initializes
             ↓
main.main() executes
             ↓
Program continues until main returns
```

If the `main()` function finishes, the program exits unless other runtime behaviour prevents termination.

---

# 7. `main` in the PlatformApp Operator

The Operator entry point is:

```text
cmd/main.go
```

Its package is:

```go
package main
```

The file is responsible for starting the Operator process.

Its high-level responsibilities include:

```text
Create a runtime scheme
        ↓
Register Kubernetes API types
        ↓
Register PlatformApp API types
        ↓
Create controller manager
        ↓
Configure metrics and health probes
        ↓
Register PlatformApp controller
        ↓
Start manager
```

The business reconciliation logic does not need to remain inside `cmd/main.go`.

It belongs in the controller package:

```text
internal/controller/
```

This separation keeps startup logic and reconciliation logic organized.

---

# 8. Packages in the PlatformApp Operator

A simplified project structure is:

```text
platformapp-operator/
├── api/
│   └── v1alpha1/
│       ├── groupversion_info.go
│       ├── platformapp_types.go
│       └── zz_generated.deepcopy.go
├── cmd/
│   └── main.go
├── internal/
│   └── controller/
│       ├── platformapp_controller.go
│       └── platformapp_controller_test.go
├── config/
├── go.mod
├── go.sum
└── Makefile
```

The important Go packages are:

| Directory | Package | Responsibility |
|---|---|---|
| `cmd` | `main` | Starts the Operator |
| `api/v1alpha1` | `v1alpha1` | Defines and registers the custom API |
| `internal/controller` | `controller` | Implements reconciliation |
| `internal/controller` test files | `controller` or `controller_test` | Tests controller behaviour |

---

# 9. Multiple Files in One Package

A package can contain multiple files.

Example:

```text
api/v1alpha1/
├── groupversion_info.go
├── platformapp_types.go
└── zz_generated.deepcopy.go
```

All these files declare:

```go
package v1alpha1
```

Go treats their package-level declarations as parts of the same package.

For example, one file can define:

```go
type PlatformApp struct {
}
```

Another file in the same package can refer to:

```go
PlatformApp
```

without importing the package again.

This is because both files belong to `v1alpha1`.

---

# 10. Imports

An import allows one package to use exported functionality from another package.

Example:

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
```

Here:

```text
Current package = main
Imported package = fmt
Used function = fmt.Println
```

The dot separates the package name from an exported identifier:

```go
fmt.Println
```

Read it as:

> Use `Println` from the `fmt` package.

---

# 11. Single and Grouped Imports

A single import can be written as:

```go
import "fmt"
```

Multiple imports are normally grouped:

```go
import (
	"context"
	"fmt"
	"time"
)
```

This is clearer than writing separate import declarations.

Go formatting tools organize import blocks consistently.

---

# 12. Standard-Library Imports

Go includes a standard library.

Examples:

```go
import (
	"context"
	"errors"
	"fmt"
	"time"
)
```

These packages are supplied with Go.

| Package | Purpose |
|---|---|
| `fmt` | Formatted text |
| `context` | Cancellation, deadlines, and request context |
| `errors` | Error inspection and construction |
| `time` | Time values, durations, and timers |
| `encoding/json` | JSON encoding and decoding |
| `net/http` | HTTP clients and servers |
| `os` | Operating-system functionality |
| `log` | Basic logging |

A standard-library package does not need to be downloaded as a separate third-party dependency.

---

# 13. Third-Party Imports

Packages outside the Go standard library are third-party or external dependencies.

Examples from Operator development:

```go
import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)
```

These imports provide Kubernetes and controller-runtime functionality.

Examples:

| Import | Purpose |
|---|---|
| `k8s.io/api/apps/v1` | Deployment and related workload APIs |
| `k8s.io/api/core/v1` | Service, Pod, Container, and other core APIs |
| `k8s.io/apimachinery/pkg/api/errors` | Kubernetes API error helpers |
| `sigs.k8s.io/controller-runtime` | Controller manager and reconcile types |
| `sigs.k8s.io/controller-runtime/pkg/client` | Kubernetes client interface |

These dependencies are declared and versioned through `go.mod`.

---

# 14. Local Project Imports

Packages within the current module can import one another.

The module path for the project is:

```text
github.com/fixnops/platformapp-operator
```

The controller package can import the local API package:

```go
import (
	platformv1alpha1 "github.com/fixnops/platformapp-operator/api/v1alpha1"
)
```

This path has two parts:

```text
Module path:
github.com/fixnops/platformapp-operator

Package directory:
api/v1alpha1
```

Combined import path:

```text
github.com/fixnops/platformapp-operator/api/v1alpha1
```

Then controller code can use:

```go
platformv1alpha1.PlatformApp
```

---

# 15. Import Aliases

An import can have an alias:

```go
import (
	appsv1 "k8s.io/api/apps/v1"
)
```

The alias is:

```text
appsv1
```

The package is then referenced as:

```go
appsv1.Deployment
```

Without an explicit alias, Go normally uses the package's declared name.

Aliases are useful when:

- The natural package name is unclear
- Multiple packages have similar names
- The alias communicates the API group and version
- The conventional Kubernetes alias is widely understood

Common Kubernetes aliases include:

```go
appsv1
corev1
metav1
apierrors
ctrl
```

Examples:

```go
appsv1.Deployment{}
corev1.Service{}
metav1.ObjectMeta{}
ctrl.Result{}
```

The alias is chosen by the importing file. It does not rename the original package globally.

---

# 16. Understanding Kubernetes Import Aliases

Consider:

```go
import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

These aliases describe different Kubernetes API areas:

```text
appsv1
├── Deployment
├── StatefulSet
└── DaemonSet

corev1
├── Pod
├── Service
├── ConfigMap
└── Secret

metav1
├── ObjectMeta
├── TypeMeta
└── Condition
```

Example:

```go
deployment := &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "nginx-demo",
		Namespace: "platformapp-demo",
	},
}
```

This uses:

- `Deployment` from `apps/v1`
- `ObjectMeta` from Kubernetes metadata APIs

---

# 17. Unused Imports

Go does not allow unused imports.

Invalid:

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Hello")
}
```

The `time` package was imported but never used.

The compiler reports an error similar to:

```text
"time" imported and not used
```

This prevents unnecessary dependencies from accumulating in source files.

The corrected program is:

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
```

---

# 18. Exported and Unexported Identifiers

An identifier is a name used in Go code.

Examples include:

- Variable names
- Function names
- Type names
- Method names
- Field names
- Constant names

Go uses capitalization to control package visibility.

## Exported identifier

A name beginning with an uppercase letter is exported:

```go
type PlatformApp struct {
}

func CreateDeployment() {
}
```

Other packages can access exported identifiers.

## Unexported identifier

A name beginning with a lowercase letter is unexported:

```go
func desiredLabels() {
}

var defaultReplicas int32
```

These are accessible only within their package.

Memory mapping:

```text
Uppercase first letter = Public outside the package
Lowercase first letter = Private to the package
```

Go documentation generally uses the terms:

```text
exported
unexported
```

rather than public and private.

---

# 19. Why `PlatformApp` Is Exported

The type name begins with uppercase:

```go
type PlatformApp struct {
}
```

This allows code in other packages to use it.

The controller package imports the API package and refers to:

```go
platformv1alpha1.PlatformApp
```

If the type were defined as:

```go
type platformApp struct {
}
```

it would be unexported and unavailable to the controller package.

The same rule applies to fields.

Exported fields:

```go
type PlatformAppSpec struct {
	Image    string
	Replicas int32
	Port     int32
}
```

Unexported fields:

```go
type PlatformAppSpec struct {
	image    string
	replicas int32
	port     int32
}
```

Serialization libraries such as `encoding/json` normally require fields to be exported before they can marshal or unmarshal them.

That is one reason Kubernetes API fields begin with uppercase letters in Go.

---

# 20. Package-Level and Function-Level Names

A name declared outside a function exists at package level:

```go
package main

const applicationName = "PlatformApp"

func main() {
}
```

A name declared inside a function has local scope:

```go
func main() {
	message := "Starting Operator"
	fmt.Println(message)
}
```

The local variable `message` exists only inside `main()`.

Detailed scope rules will be covered in Chapter 4.

For now:

```text
Outside a function = package-level declaration
Inside a function  = local declaration
```

---

# 21. What Is a Go Module?

A module is a collection of related Go packages that are versioned together.

In practical terms, a module represents the Go project.

A module normally contains:

```text
go.mod
```

Example:

```text
platformapp-operator/
├── go.mod
├── go.sum
├── cmd/
├── api/
└── internal/
```

The module includes the Go packages located under its directory, subject to rules involving nested modules and excluded directories.

Memory mapping:

```text
Repository = Git source-code location
Module     = Go dependency and package boundary
Package    = Related Go files in one directory
File       = Individual source document
```

A Git repository can contain one or more modules, although many projects use one module per repository.

---

# 22. Creating a Module

A new module can be initialized using:

```bash
go mod init MODULE_PATH
```

Example:

```bash
go mod init github.com/fixnops/example
```

This creates:

```text
go.mod
```

A minimal `go.mod` might contain:

```go
module github.com/fixnops/example

go 1.27
```

The `module` line defines the base import path:

```text
github.com/fixnops/example
```

The `go` line declares the Go language/toolchain compatibility level used by the module.

The exact contents may vary with the Go version and project configuration.

---

# 23. The PlatformApp Module Path

The confirmed module path is:

```text
github.com/fixnops/platformapp-operator
```

The project `go.mod` therefore begins with a module declaration similar to:

```go
module github.com/fixnops/platformapp-operator
```

This module path becomes the base path for internal imports.

Examples:

```text
github.com/fixnops/platformapp-operator/api/v1alpha1
github.com/fixnops/platformapp-operator/internal/controller
```

The module path does not need to be repeatedly written inside packages belonging to the same directory. It is mainly used when importing one package from another.

---

# 24. Understanding `go.mod`

A simplified `go.mod` looks like:

```go
module github.com/fixnops/platformapp-operator

go 1.27

require (
	k8s.io/api v0.x.y
	k8s.io/apimachinery v0.x.y
	sigs.k8s.io/controller-runtime v0.x.y
)
```

The important sections are:

## Module declaration

```go
module github.com/fixnops/platformapp-operator
```

Defines the module's import path.

## Go version

```go
go 1.27
```

Defines the Go language version expected by the module.

## Requirements

```go
require (
	...
)
```

Lists dependencies and their selected versions.

The actual Operator project may contain many direct and indirect dependencies.

Do not manually invent dependency versions. Use the versions selected by the project tooling and Go module commands.

---

# 25. Direct and Indirect Dependencies

A direct dependency is imported by the project's source code.

Example:

```go
import ctrl "sigs.k8s.io/controller-runtime"
```

An indirect dependency is needed by another dependency.

The module file may mark these as:

```go
example.com/some/module v1.2.3 // indirect
```

A simplified dependency tree might be:

```text
PlatformApp Operator
        ↓
controller-runtime
        ↓
client-go
        ↓
additional libraries
```

The Operator may not import every transitive dependency directly, but those packages are still required to build the complete dependency graph.

---

# 26. What Is `go.sum`?

`go.sum` contains cryptographic checksums for downloaded module versions.

It helps Go verify that dependency content matches the expected content.

A simplified relationship is:

```text
go.mod
    Declares dependencies and selected versions

go.sum
    Records checksums for dependency content
```

Both files should normally be committed to Git.

Do not manually edit `go.sum`.

Go updates it automatically when module dependencies are downloaded or changed.

---

# 27. Important Go Module Commands

## Display module information

```bash
go env GOMOD
```

This shows the active `go.mod` file.

## Download dependencies

```bash
go mod download
```

This downloads required module content.

## Clean module requirements

```bash
go mod tidy
```

This:

- Adds missing required modules
- Removes unused requirements
- Updates module metadata where necessary

Review changes after running it.

## Display all modules

```bash
go list -m all
```

## Explain why a module is required

```bash
go mod why MODULE_PATH
```

Example:

```bash
go mod why sigs.k8s.io/controller-runtime
```

## Display the dependency graph

```bash
go mod graph
```

Large projects can produce extensive output.

---

# 28. `go get` and Dependency Changes

`go get` can add or change module dependencies.

Example form:

```bash
go get example.com/module@version
```

Dependency changes can affect:

- `go.mod`
- `go.sum`
- Transitive dependencies
- Compatibility with Kubernetes libraries

In an Operator project, do not upgrade Kubernetes or controller-runtime dependencies casually.

Kubernetes libraries often have coordinated versions. An arbitrary update can introduce incompatibilities.

Use this process:

```text
Understand why the dependency change is required
          ↓
Check compatibility
          ↓
Run the appropriate Go command
          ↓
Review go.mod and go.sum
          ↓
Run formatting and tests
          ↓
Build the project
```

---

# 29. Package Import Path Versus Package Name

These two concepts are related but different.

Import path:

```text
sigs.k8s.io/controller-runtime/pkg/client
```

Package name:

```text
client
```

Code imports the path:

```go
import "sigs.k8s.io/controller-runtime/pkg/client"
```

Then uses the package name:

```go
client.Client
```

With an alias:

```go
import crclient "sigs.k8s.io/controller-runtime/pkg/client"
```

The code would use:

```go
crclient.Client
```

The alias changes how the current file refers to the package. It does not change the actual package.

---

# 30. The `internal` Directory

The Operator contains:

```text
internal/controller/
```

Go gives the `internal` directory a special meaning.

Packages inside an `internal` directory can only be imported by code within the allowed parent directory tree.

Example:

```text
github.com/fixnops/platformapp-operator/internal/controller
```

This package is intended for internal use by the `platformapp-operator` module.

Another unrelated project should not import it as a reusable public library.

This supports encapsulation at the project level.

Memory mapping:

```text
api/v1alpha1       = API types that other relevant packages use
internal/controller = Project-internal implementation
cmd/main.go        = Executable entry point
```

---

# 31. The `cmd` Directory

Go projects commonly place executable entry points under:

```text
cmd/
```

The Operator uses:

```text
cmd/main.go
```

Large repositories may provide multiple commands:

```text
cmd/
├── operator/
│   └── main.go
├── cli/
│   └── main.go
└── migration-tool/
    └── main.go
```

Each executable package contains:

```go
package main

func main() {
}
```

The `cmd` convention makes executable entry points easy to locate.

---

# 32. API Package Responsibilities

The package:

```text
api/v1alpha1
```

contains the custom resource API.

Its responsibilities include:

- Defining `PlatformApp`
- Defining `PlatformAppList`
- Defining `PlatformAppSpec`
- Defining `PlatformAppStatus`
- Declaring API markers
- Registering API types
- Supporting generated deep-copy behaviour

A simplified API type might look like:

```go
package v1alpha1

type PlatformAppSpec struct {
	Image    string `json:"image"`
	Replicas int32  `json:"replicas"`
	Port     int32  `json:"port"`
}
```

The package name reflects the API version:

```text
v1alpha1
```

---

# 33. Controller Package Responsibilities

The package:

```text
internal/controller
```

contains controller behaviour.

Its responsibilities include:

- Receiving reconciliation requests
- Reading `PlatformApp`
- Calculating desired Deployment state
- Calculating desired Service state
- Creating or updating child resources
- Managing owner references
- Updating status
- Returning errors or requeue decisions
- Registering watches

A simplified controller type might look like:

```go
package controller

type PlatformAppReconciler struct {
	client.Client
}
```

The controller package imports the API package because it needs to read `PlatformApp` objects.

---

# 34. Package Dependency Direction

The typical direction is:

```text
cmd/main
   ├── imports API package
   └── imports controller package

controller package
   ├── imports API package
   ├── imports Kubernetes API packages
   └── imports controller-runtime

API package
   ├── imports Kubernetes metadata/runtime packages
   └── should not depend on controller implementation
```

This keeps the API definition separate from the reconciliation implementation.

An undesirable design would make the API package depend on controller implementation details.

---

# 35. Avoiding Import Cycles

Go does not allow package import cycles.

Invalid dependency:

```text
package A imports package B
package B imports package A
```

This creates:

```text
A → B → A
```

Go rejects the cycle.

Import cycles usually indicate that responsibilities need to be reorganized.

A healthy simplified Operator structure is:

```text
main → controller → API
  └──────────────→ API
```

The API package does not import the controller package, so there is no cycle.

---

# 36. Special Import Forms

Some special forms may appear in Go code.

## Alias import

```go
import appsv1 "k8s.io/api/apps/v1"
```

Use the package through `appsv1`.

## Blank import

```go
import _ "example.com/driver"
```

The blank identifier imports a package for its initialization side effects without directly referring to its exported names.

This is less common in basic Operator controller code and should only be used deliberately.

## Dot import

```go
import . "example.com/package"
```

A dot import makes exported names available without a package prefix.

It can reduce clarity and should generally be avoided in normal application code. Some test frameworks may use this style.

---

# 37. The `init()` Function

A package may define:

```go
func init() {
}
```

`init()` runs automatically during package initialization before `main()`.

A package can contain multiple `init()` functions.

Initialization broadly occurs before normal program execution:

```text
Initialize imported packages
          ↓
Run package-level initializers
          ↓
Run init() functions
          ↓
Run main()
```

`init()` can be useful for package registration, but excessive hidden initialization makes code harder to understand and test.

In Kubernetes API packages, generated or scaffolded code may use initialization to register types with a scheme builder.

Do not add `init()` functions casually.

---

# 38. What Is a Scheme?

This chapter focuses on packages, but the Operator's package organization exposes an important concept: the runtime scheme.

The scheme teaches the Kubernetes Go client how Go types map to Kubernetes API identities.

For example:

```text
Go type:
platformv1alpha1.PlatformApp

Kubernetes identity:
apps.fixnops.com/v1alpha1, Kind=PlatformApp
```

The API package provides registration functionality, and `cmd/main.go` adds those types to the manager's scheme.

A simplified conceptual flow is:

```go
platformv1alpha1.AddToScheme(scheme)
```

This means:

> Register the PlatformApp API types with this runtime scheme.

The scheme will be explained in depth in Chapter 12.

---

# 39. How the Go Tool Finds Packages

When Go encounters:

```go
import (
	platformv1alpha1 "github.com/fixnops/platformapp-operator/api/v1alpha1"
)
```

it determines that:

1. The current module is `github.com/fixnops/platformapp-operator`.
2. The import begins with the current module path.
3. The package exists locally under `api/v1alpha1`.
4. The package's exported identifiers can be used.

For an external import, Go uses module metadata to find the required dependency version.

---

# 40. Build Package Versus Build Module

A module can contain multiple packages.

When running:

```bash
go build .
```

the dot refers to the current package.

When running:

```bash
go build ./...
```

the pattern means all packages recursively under the current module directory.

Similarly:

```bash
go test ./...
```

tests all matching packages.

This is why `./...` appears frequently in Go development.

Memory mapping:

```text
.     = Current package
./... = Current package and descendant packages
```

---

# 41. Generated Code Versus Handwritten Code

The Operator repository contains both handwritten and generated Go code.

## Handwritten code

Examples:

```text
cmd/main.go
api/v1alpha1/platformapp_types.go
internal/controller/platformapp_controller.go
```

Developers intentionally edit these files.

## Generated code

Example:

```text
api/v1alpha1/zz_generated.deepcopy.go
```

This file is generated from API types and markers.

The project regenerates it using:

```bash
make generate
```

CRD YAML is regenerated using:

```bash
make manifests
```

Do not manually edit generated files unless the project explicitly requires it.

The standard workflow is:

```text
Edit source API types or markers
          ↓
Run generation command
          ↓
Review generated changes
          ↓
Run tests
```

---

# 42. Package Documentation

A package can have documentation comments.

Example:

```go
// Package controller implements Kubernetes controllers for PlatformApp.
package controller
```

An exported type should usually have a comment beginning with its name:

```go
// PlatformAppReconciler reconciles PlatformApp resources.
type PlatformAppReconciler struct {
}
```

This improves:

- Generated documentation
- Editor assistance
- Code review
- Maintainability
- Static-analysis results

Comments should explain purpose and behaviour clearly.

---

# 43. Practical Example: Multiple Packages

The following is a conceptual learning project:

```text
application/
├── go.mod
├── main.go
└── platform/
    └── describe.go
```

`go.mod`:

```go
module github.com/fixnops/application

go 1.27
```

`platform/describe.go`:

```go
package platform

import "fmt"

// Describe returns a description of an application.
func Describe(name string, replicas int32) string {
	return fmt.Sprintf(
		"application %s requires %d replicas",
		name,
		replicas,
	)
}
```

`main.go`:

```go
package main

import (
	"fmt"

	"github.com/fixnops/application/platform"
)

func main() {
	description := platform.Describe("nginx-demo", 2)
	fmt.Println(description)
}
```

The flow is:

```text
main package
      ↓ imports
platform package
      ↓ calls
platform.Describe
      ↓ returns
description string
```

`Describe` is exported because it starts with uppercase `D`.

If it were named:

```go
func describe(...) {
}
```

the `main` package could not call it.

---

# 44. Connection to the PlatformApp Operator

The real project follows the same principle on a larger scale.

Conceptually:

```go
package main

import (
	platformv1alpha1 "github.com/fixnops/platformapp-operator/api/v1alpha1"
	"github.com/fixnops/platformapp-operator/internal/controller"
)
```

`main` uses API registration:

```go
platformv1alpha1.AddToScheme(scheme)
```

It also creates the reconciler:

```go
reconciler := &controller.PlatformAppReconciler{
	Client: manager.GetClient(),
}
```

The exact project code may be structured differently, but the package relationships remain:

```text
main starts
   ↓
API types registered
   ↓
Controller registered
   ↓
Manager starts
   ↓
Reconciliation events processed
```

---

# 45. Common Beginner Mistakes

## Mistake 1: Different packages in the same normal directory

Incorrect structure:

```go
// file1.go
package main
```

```go
// file2.go
package controller
```

Normal non-test Go files in one directory should belong to the same package.

## Mistake 2: Importing a package without using it

Go rejects unused imports.

## Mistake 3: Using an unexported identifier from another package

This is inaccessible outside its package:

```go
func describe() {
}
```

Export it when appropriate:

```go
func Describe() {
}
```

## Mistake 4: Confusing package name with import path

Import path:

```text
sigs.k8s.io/controller-runtime/pkg/client
```

Package name:

```text
client
```

## Mistake 5: Manually editing `go.sum`

`go.sum` is maintained by Go tooling.

## Mistake 6: Adding dependencies without reviewing them

Dependency changes can affect compatibility and security.

## Mistake 7: Creating an import cycle

Go does not permit circular package dependencies.

## Mistake 8: Placing all logic in `main.go`

Keep startup logic, API definitions, and controller behaviour separated.

## Mistake 9: Editing generated files

Change the source definition or marker, then regenerate.

## Mistake 10: Running commands from the wrong module

Check:

```bash
go env GOMOD
```

before performing module operations.

---

# Documentation Exercises

These exercises are recorded now and will be performed during the practical learning phase.

## Exercise 1: Identify project organization

Inspect the Operator repository and record:

1. Module path
2. Executable package
3. API package
4. Controller package
5. Generated Go files
6. Handwritten Go files

## Exercise 2: Classify imports

Select imports from `platformapp_controller.go` and classify each as:

- Standard library
- Local project package
- Kubernetes package
- controller-runtime package
- Test-only package

## Exercise 3: Exported versus unexported

For each name, identify whether it is exported:

```text
PlatformApp
PlatformAppSpec
Reconcile
desiredDeployment
labelsForPlatformApp
Client
replicas
```

## Exercise 4: Trace an import

Trace:

```go
platformv1alpha1.PlatformApp
```

Record:

1. The import path
2. The local alias
3. The package directory
4. The file defining `PlatformApp`
5. Why the type must be exported

## Exercise 5: Inspect module metadata

During the practical phase, run:

```bash
go env GOMOD
go list -m
go list -m all
go mod why sigs.k8s.io/controller-runtime
```

Explain what each command reports.

---

# Interview Questions

## Question 1: What is a package in Go?

Suggested answer:

> A package groups related Go files, types, functions, and variables. Files in the same directory normally belong to the same package. Packages provide code organization, reuse, and visibility boundaries.

## Question 2: What is a module?

Suggested answer:

> A module is a collection of related Go packages versioned together. It is identified by the module path in `go.mod`, which also defines dependency requirements and the Go version used by the project.

## Question 3: What is `go.mod`?

Suggested answer:

> `go.mod` defines the module path, Go version, and required module dependencies. It is the primary module configuration file.

## Question 4: What is `go.sum`?

Suggested answer:

> `go.sum` stores cryptographic checksums for downloaded dependency module versions. Go uses it to verify dependency content. It is generated by Go tooling and normally committed to Git.

## Question 5: What is the difference between exported and unexported names?

Suggested answer:

> An identifier beginning with an uppercase letter is exported and can be accessed from other packages. A lowercase identifier is unexported and is accessible only within its own package.

## Question 6: Why are Kubernetes Go types written as `appsv1.Deployment`?

Suggested answer:

> `appsv1` is an import alias for the Kubernetes `apps/v1` API package, and `Deployment` is an exported type from that package. The alias makes the API group and version clear.

## Question 7: What does `./...` mean in Go commands?

Suggested answer:

> It represents the current package and all descendant packages. For example, `go test ./...` runs tests across packages recursively under the current module location.

## Question 8: What is the purpose of `internal/controller`?

Suggested answer:

> The `internal/controller` package holds the project's controller implementation. Go's `internal` rule prevents unrelated external modules from importing that implementation as a public library.

## Question 9: Why should generated files not be manually edited?

Suggested answer:

> Manual changes will normally be overwritten the next time generation runs. The correct approach is to edit the source types or markers and regenerate the derived files.

## Question 10: What happens in `cmd/main.go`?

Suggested answer:

> It is the Operator's executable entry point. It creates the runtime scheme and manager, registers Kubernetes and custom API types, configures metrics and probes, registers the reconciler, and starts the manager.

## Question 11: Why does Go reject unused imports?

Suggested answer:

> It keeps dependencies and source code clean and prevents unused code from accumulating. Every imported package must be intentionally used.

## Question 12: What is an import cycle?

Suggested answer:

> An import cycle occurs when package A depends on package B and package B directly or indirectly depends on package A. Go rejects circular package dependencies, so responsibilities must be reorganized to produce a one-directional dependency structure.

---

# Chapter Checkpoint

Do not mark this chapter complete until you can explain:

- [ ] What a Go source file is
- [ ] What a package is
- [ ] Why files in one directory normally use the same package
- [ ] Why `main` is a special package
- [ ] How imports work
- [ ] Standard-library versus third-party imports
- [ ] Local project imports
- [ ] Import aliases such as `appsv1`
- [ ] Exported versus unexported identifiers
- [ ] What a Go module is
- [ ] The purpose of `go.mod`
- [ ] The purpose of `go.sum`
- [ ] Direct and indirect dependencies
- [ ] The meaning of `.` and `./...`
- [ ] The purpose of `cmd`
- [ ] The purpose of `internal`
- [ ] The purpose of `api/v1alpha1`
- [ ] Why import cycles are rejected
- [ ] Generated versus handwritten files
- [ ] How packages are organized in the PlatformApp Operator

## Memory Map

```text
Module
└── Complete Go project and dependency boundary

Package
└── Related Go files in one directory

File
└── Individual .go source file

main package
└── Produces an executable

main function
└── Program entry point

Import
└── Use exported functionality from another package

Uppercase identifier
└── Exported outside the package

Lowercase identifier
└── Available only inside the package

go.mod
└── Module path, Go version, and dependency requirements

go.sum
└── Dependency-content checksums

cmd
└── Executable entry points

internal
└── Project-internal implementation

api/v1alpha1
└── Custom Kubernetes API definitions
```

## Operator Package Flow

```text
cmd/main.go
├── imports api/v1alpha1
├── imports internal/controller
└── starts manager
          ↓
internal/controller
├── imports api/v1alpha1
├── imports Kubernetes APIs
└── implements reconciliation
          ↓
api/v1alpha1
└── defines PlatformApp API types
```

## Next Chapter

Continue to:

[Chapter 3: Variables, Constants, Types, and Zero Values](03-variables-constants-types-and-zero-values.md.md)
