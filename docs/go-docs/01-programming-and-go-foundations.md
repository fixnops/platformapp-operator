# Chapter 1: Programming and Go Foundations

## Chapter Goal

This chapter explains programming from the absolute beginning.

After completing it, you should understand:

- What a computer program is
- What source code is
- Why programming languages exist
- What compilation means
- What a binary executable is
- What Go is
- Why Kubernetes uses Go
- How a basic Go program executes
- How to run and build a Go program
- How this relates to a Kubernetes Operator

No previous programming knowledge is assumed.

---

## 1. What Is a Computer Program?

A computer program is a set of instructions given to a computer.

For example, imagine giving these instructions to a person:

```text
1. Read the application name.
2. Read the requested replica count.
3. Check how many replicas currently exist.
4. If the requested and current values differ, correct the difference.
5. Report the result.
```

A computer program expresses similar instructions in a programming language.

A simplified Operator program might follow this logic:

```text
Read PlatformApp
Check Deployment
Compare desired replicas with current replicas
Update Deployment when they differ
Update PlatformApp status
```

The computer follows instructions exactly. It does not understand the developer's intention unless that intention is correctly expressed in code.

### Real-life analogy

A cooking recipe contains:

- Inputs: ingredients
- Instructions: cooking steps
- Conditions: if the sauce is too thick, add water
- Output: prepared food

A program contains:

- Inputs: data
- Instructions: functions and statements
- Conditions: `if` and `switch`
- Output: a result or change

A Kubernetes Operator is a program whose primary input is Kubernetes API data and whose output is changes to Kubernetes resources.

---

## 2. What Is a Programming Language?

Computer processors ultimately execute machine instructions represented using binary values.

Writing large applications directly in machine instructions would be extremely difficult. Programming languages allow humans to express instructions in a structured and readable form.

Examples of programming languages include:

- Go
- Python
- Java
- C
- C++
- Rust
- JavaScript

The same general task can often be implemented using different languages.

For Kubernetes Operator development, Go is commonly used because Kubernetes and the main controller libraries are written in Go.

---

## 3. What Is Source Code?

Source code is the human-readable text written in a programming language.

Example Go source code:

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello from Go")
}
```

This source code is saved in a file with the `.go` extension:

```text
main.go
```

The source file is readable by a developer, but the processor does not directly execute the text as written.

The Go toolchain must process it first.

---

## 4. Compiler and Executable

Go is a compiled language.

The Go compiler converts Go source code into a machine-executable file.

```text
main.go
   ↓
Go compiler
   ↓
Executable binary
```

A binary is a compiled executable program.

For an Operator, the complete flow is:

```text
Go source files
      ↓
Go compiler
      ↓
Operator binary
      ↓
Container image
      ↓
Operator pod
      ↓
Controller runs inside Kubernetes
```

### Why compile a program?

Compilation provides several benefits:

- Syntax and type errors can be detected before execution.
- The result can run as a standalone executable.
- Compiled Go applications are usually fast.
- A Go binary is convenient to place inside a container image.
- Many mistakes are detected early by the compiler.

### Example compiler error

The following code is invalid because the closing quotation mark is missing:

```go
fmt.Println("Hello)
```

The compiler refuses to build it and reports an error.

This is useful. A compiler error is not an enemy; it tells the developer that the program cannot be safely built in its current form.

---

## 5. Compiled Language Versus Interpreted Language

A simplified comparison is:

```text
Compiled:
Source code → Compiler → Executable → Run

Interpreted:
Source code → Interpreter reads and executes it
```

Go is primarily compiled.

Python is commonly executed through a Python interpreter.

This distinction is simplified because modern language implementations may use multiple techniques. For our Operator learning, the important point is:

> Go source code is compiled into an executable binary.

---

## 6. What Is Go?

Go is an open-source programming language originally developed at Google.

It is commonly used for:

- Cloud-native platforms
- Kubernetes components
- Infrastructure tools
- Network services
- APIs
- Command-line tools
- Automation
- Distributed systems

Important characteristics of Go include:

### Statically typed

Every variable has a type known to the compiler.

```go
var replicas int32 = 2
```

Here, `replicas` has the type `int32`.

The following assignment is invalid:

```go
replicas = "two"
```

A string cannot be assigned to an `int32` variable.

### Compiled

Go source code is compiled into an executable binary.

### Garbage collected

Go automatically manages memory that is no longer needed.

Developers still need to understand values and pointers, but they normally do not manually free memory.

### Concurrent

Go provides goroutines and channels for concurrent work.

Controller-runtime uses concurrent workers internally to process reconciliation requests.

### Simple language design

Go deliberately provides a relatively small set of language features.

This encourages:

- Readable code
- Consistent formatting
- Explicit error handling
- Composition
- Simple control flow

---

## 7. Why Kubernetes Uses Go

Kubernetes requires software that can:

- Run continuously
- Handle many API requests
- Work concurrently
- Communicate over networks
- Run efficiently inside containers
- Be distributed as executable binaries
- Work across multiple operating systems and processor architectures

Go is well suited to these requirements.

Major cloud-native projects written primarily in Go include:

- Kubernetes
- etcd
- Prometheus
- Terraform
- Docker components
- Operator SDK
- controller-runtime

Learning Go allows an Operator developer to use the same types and libraries used throughout the Kubernetes ecosystem.

---

## 8. Go in the PlatformApp Operator

The `PlatformApp` Operator uses Go for several responsibilities:

```text
Go API types
    Define PlatformApp Spec and Status
             ↓
Go controller
    Watches PlatformApp resources
             ↓
Go reconciliation logic
    Compares desired and actual state
             ↓
Kubernetes Go client
    Creates or updates Deployment and Service
             ↓
Go status logic
    Reports the observed result
```

Examples of Go files in the Operator include:

```text
api/v1alpha1/platformapp_types.go
internal/controller/platformapp_controller.go
cmd/main.go
```

The Operator SDK created the initial project structure, but the behaviour is implemented in Go.

---

## 9. Verify the Go Environment

Check the installed Go version:

```bash
go version
```

Example:

```text
go version go1.27.1 darwin/arm64
```

This output means:

```text
go1.27.1    = Installed Go version
darwin      = macOS
arm64       = ARM64 processor architecture
```

Find the Go executable:

```bash
which go
```

Inspect the Go environment:

```bash
go env
```

Some useful values include:

```bash
go env GOOS
go env GOARCH
go env GOMOD
go env GOPATH
```

Their meanings are:

| Variable | Meaning |
|---|---|
| `GOOS` | Target operating system |
| `GOARCH` | Target processor architecture |
| `GOMOD` | Active `go.mod` file |
| `GOPATH` | Go workspace and cache location |

Do not try to memorize every value displayed by `go env`. Learn the values as they become relevant.

---

## 10. Create the First Go Program

From the repository root, create a separate exercise directory:

```bash
cd ~/operator-build
mkdir -p go-learning/chapter-01
cd go-learning/chapter-01
```

Initialize a Go module:

```bash
go mod init github.com/fixnops/go-operator-learning/chapter-01
```

Expected output:

```text
go: creating new go.mod: module github.com/fixnops/go-operator-learning/chapter-01
```

Create the source file:

```bash
touch main.go
```

The directory should now contain:

```text
chapter-01/
├── go.mod
└── main.go
```

Open it:

```bash
code .
```

Add the following to `main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("Starting Go for Operator development")
}
```

Format the program:

```bash
go fmt ./...
```

Run it:

```bash
go run .
```

Expected output:

```text
Starting Go for Operator development
```

---

## 11. Understand the Program Line by Line

### Package declaration

```go
package main
```

Every Go source file belongs to a package.

A package groups related Go code.

`main` is a special package name. It tells Go that this package can produce an executable program.

### Import declaration

```go
import "fmt"
```

The program wants to use functionality from the `fmt` package.

`fmt` belongs to the Go standard library and provides formatted input and output functions.

### Main function

```go
func main()
```

`func` means function.

`main` is a special function name inside `package main`.

Program execution begins from `main()`.

### Function body

```go
{
	fmt.Println("Starting Go for Operator development")
}
```

The opening and closing braces define the function body.

The statements inside the braces are executed when the function runs.

### Calling another function

```go
fmt.Println(...)
```

This means:

```text
fmt       = package
Println   = exported function in that package
.         = access something from the package
```

`Println` prints the supplied value followed by a new line.

### String value

```go
"Starting Go for Operator development"
```

Text inside double quotation marks is a string.

---

## 12. The Program Execution Flow

When this command runs:

```bash
go run .
```

Go performs a sequence similar to:

```text
Find the current module
       ↓
Find Go source files in the current package
       ↓
Check the syntax and types
       ↓
Compile the program
       ↓
Run the temporary executable
       ↓
Display the program output
```

The `.` means the current package in the current directory.

---

## 13. `go run` Versus `go build`

### `go run`

```bash
go run .
```

This compiles and immediately runs the program.

It is convenient during learning and development.

### `go build`

```bash
go build .
```

This creates a persistent executable.

List the files afterward:

```bash
ls -lh
```

On macOS or Linux, run the binary:

```bash
./chapter-01
```

Expected output:

```text
Starting Go for Operator development
```

The important difference is:

```text
go run   = compile temporarily and execute
go build = compile and keep the executable
```

An Operator container ultimately runs a compiled binary, not the original `.go` text files.

---

## 14. Formatting Go Code

Go includes an official formatter.

Run:

```bash
go fmt ./...
```

Suppose the source code is poorly spaced:

```go
func main(){fmt.Println("Hello")}
```

`go fmt` changes it to the standard style:

```go
func main() {
	fmt.Println("Hello")
}
```

This gives Go projects a consistent coding style.

The Operator project provides:

```bash
make fmt
```

That command formats the project code and performs related formatting checks defined by the project Makefile.

---

## 15. Comments

Comments are notes for developers. They are not executed as program instructions.

### Single-line comment

```go
// Start the example program.
fmt.Println("Starting Go")
```

### Multi-line comment

```go
/*
This is a learning program.
It demonstrates basic Go execution.
*/
```

Use comments to explain important reasons or non-obvious decisions.

Avoid comments that merely repeat the code.

Unhelpful:

```go
// Print hello.
fmt.Println("Hello")
```

More useful:

```go
// This message confirms that the controller process reached startup.
fmt.Println("Controller starting")
```

---

## 16. Exported and Unexported Names: First Introduction

Go uses capitalization to control whether a name is accessible outside its package.

Uppercase names are exported:

```go
fmt.Println
```

`Println` starts with uppercase `P`, so other packages can access it.

Lowercase names are unexported:

```go
func describeApp() {
}
```

`describeApp` can only be accessed from within its own package.

We will examine this properly in Chapter 2.

For now, remember:

```text
Uppercase first letter = exported
Lowercase first letter = package-private
```

---

## 17. Practical Experiment: Change the Program

Change `main()` to:

```go
func main() {
	fmt.Println("PlatformApp Operator")
	fmt.Println("API: apps.fixnops.com/v1alpha1")
	fmt.Println("Kind: PlatformApp")
}
```

Run:

```bash
go fmt ./...
go run .
```

Expected output:

```text
PlatformApp Operator
API: apps.fixnops.com/v1alpha1
Kind: PlatformApp
```

This demonstrates that statements execute from top to bottom.

```text
First Println
      ↓
Second Println
      ↓
Third Println
```

---

## 18. Learning from Compiler Errors

Compiler errors are an essential part of development.

Create each error deliberately, observe it, and then repair it.

### Experiment 1: Missing quotation mark

Invalid:

```go
fmt.Println("PlatformApp Operator)
```

Run:

```bash
go run .
```

Read the error carefully, then restore the quotation mark.

### Experiment 2: Misspelled function name

Invalid:

```go
fmt.Printline("PlatformApp Operator")
```

Go is case-sensitive and requires the exact function name:

```go
fmt.Println("PlatformApp Operator")
```

### Experiment 3: Remove the import

Remove:

```go
import "fmt"
```

The program cannot use `fmt.Println` without importing `fmt`.

### Experiment 4: Unused import

Keep:

```go
import "fmt"
```

But remove all uses of `fmt`.

Go reports that `fmt` was imported but not used.

This strictness prevents unused dependencies and dead code from accumulating silently.

### Important lesson

Do not react to an error by randomly changing code.

Use this process:

```text
Read the complete error
       ↓
Identify the file and line
       ↓
Understand what Go expected
       ↓
Make one correction
       ↓
Format and run again
```

---

## 19. Source Code to Operator Pod

The small learning program and the Operator follow the same fundamental process.

### Learning program

```text
main.go
   ↓
go build
   ↓
chapter-01 binary
   ↓
Run locally
```

### Operator

```text
Multiple Go source files
       ↓
make build
       ↓
Manager/operator binary
       ↓
Docker build
       ↓
Container image
       ↓
Kubernetes Deployment
       ↓
Operator pod
```

The Operator contains more packages and dependencies, but it is still a compiled Go program.

---

## 20. Common Beginner Mistakes

### Mistake 1: Wrong capitalization

Incorrect:

```go
fmt.println("Hello")
```

Correct:

```go
fmt.Println("Hello")
```

Go is case-sensitive.

### Mistake 2: Missing import

Using `fmt.Println` requires:

```go
import "fmt"
```

### Mistake 3: Unused import

Go rejects imports that are not used.

### Mistake 4: Running from the wrong directory

Run:

```bash
pwd
ls
```

Confirm that the directory contains `go.mod` and `main.go`.

### Mistake 5: Editing generated Operator files manually

Learning programs are written manually.

Files such as this one are generated:

```text
api/v1alpha1/zz_generated.deepcopy.go
```

Generated files should normally be regenerated using the project's generation commands.

### Mistake 6: Memorizing without running

Reading code creates familiarity. Running and modifying code creates understanding.

---

# Practical Exercises

## Exercise 1: Personalize the output

Make the program print:

```text
Learner: Dinesh
Goal: Build Kubernetes Operators with Go
Project: PlatformApp
```

Requirement: use three separate `fmt.Println` statements.

## Exercise 2: Add comments

Add:

- One useful single-line comment
- One useful multi-line comment

The comments should explain the program's purpose rather than repeating individual statements.

## Exercise 3: Build the executable

Run:

```bash
go fmt ./...
go run .
go build .
ls -lh
./chapter-01
```

Confirm that `go run .` and the compiled binary produce the same result.

## Exercise 4: Observe a compiler error

Deliberately misspell:

```go
fmt.Println
```

Run:

```bash
go run .
```

Record:

1. The error message
2. What caused it
3. How you fixed it

Restore the working program afterward.

## Exercise 5: Operator connection

Answer in your own words:

1. Is an Operator fundamentally a program?
2. What converts the Go source code into an executable?
3. Does the Operator pod directly execute `.go` source files?
4. What does the Operator container execute?
5. Why is `package main` important?

---

# Interview Questions

## Question 1: What is Go?

Suggested answer:

> Go is a statically typed, compiled programming language commonly used for cloud-native systems, APIs, infrastructure automation, and concurrent services. Kubernetes and controller-runtime are written in Go, which makes it a natural choice for Kubernetes Operator development.

## Question 2: What does compiled language mean?

Suggested answer:

> It means source code is processed by a compiler and converted into an executable form before it runs. In Go, `go build` compiles the source code into a binary.

## Question 3: Why is Go used for Kubernetes?

Suggested answer:

> Go produces efficient standalone binaries, has strong concurrency support, provides static type checking, works well for networking and distributed systems, and supports multiple platforms. Kubernetes and its main controller libraries are also implemented in Go.

## Question 4: What is `package main`?

Suggested answer:

> `package main` defines an executable Go package. When it contains a `main()` function, that function becomes the entry point of the executable.

## Question 5: What is the difference between `go run` and `go build`?

Suggested answer:

> `go run` compiles and immediately executes the program using a temporary binary. `go build` compiles the program and retains the executable file.

## Question 6: What is `go fmt`?

Suggested answer:

> `go fmt` automatically formats Go source code using the standard Go style. It keeps formatting consistent across developers and projects.

---

# Chapter Checkpoint

Do not mark this chapter complete until you can do all of the following:

- [ ] Explain what a computer program is.
- [ ] Explain source code, compiler, and executable binary.
- [ ] Explain why Kubernetes uses Go.
- [ ] Create a Go module.
- [ ] Write a valid `main.go`.
- [ ] Run the program using `go run .`.
- [ ] Build it using `go build .`.
- [ ] Run the compiled executable.
- [ ] Explain `package main`.
- [ ] Explain `import "fmt"`.
- [ ] Explain `func main()`.
- [ ] Use `go fmt`.
- [ ] Read and correct a basic compiler error.
- [ ] Explain how Go source becomes an Operator pod.

## Memory Map

```text
Program     = Instructions for a computer
Source code = Human-readable program text
Compiler    = Converts source code into executable instructions
Binary      = Compiled executable file
Package     = Group of related Go code
main        = Executable package and starting function
Import      = Use functionality from another package
go run      = Compile temporarily and run
go build    = Compile and retain the executable
go fmt      = Apply standard Go formatting
Operator    = Go program that manages Kubernetes resources
```

## Next Chapter

Continue to:

[Chapter 2: Go Program Structure, Packages, and Modules](02-go-program-structure-packages-and-modules.md)