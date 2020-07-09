# GitLab Runner style guidelines

This document is meant to be an extension of the existing [GitLab Go standards and style guidelines](https://docs.gitlab.com/ee/development/go_guide).

## Guidelines

### Unexported interfaces

It's OK to use unexported interfaces when it doesn't make sense to export them outside of the package.
They are also sometimes needed to generate mocks. Methods in unexported interfaces should always be exported.
This makes a distinct difference between API and helper methods.
This approach also, allows us to easily export previously unexported interfaces.

**Avoid:**

```golang
type writer interface {
    write(b []byte) error
}
```

**Prefer:**

```golang
type writer interface {
    Write(b []byte) error
}
```

### Longer function definitions/calls

On longer function definitions/calls prefer to put all arguments on their own lines.
The [lll](http://gitlab.com/gitlab-org/gitlab-runner/blob/7b6b46203bd9ae84696e27d77afd293f385451aa/.golangci.yml#L73-73)
linter will tell you when the line is too long and should be split up.

**Avoid:**

```golang
err = connectToServer(
    connection.Hostname, connection.Port,
    connection.Username, connection.PrivateKey,
)
```

**Prefer:**

```golang
err = connectToServer(
    connection.Hostname,
    connection.Port,
    connection.Username,
    connection.PrivateKey,
)
```

### Amount of input/output arguments

Limit the amount of input/output arguments of functions to a reasonable number.
For input arguments a maximum number of 4 is preferred. For output arguments 2 are preferred
(one of them being an error) and in rare cases, 3 would be OK. If there's a need for more arguments consider
splitting the function or if not possible create an input/output struct. E.g.:

**Avoid:**

```golang
func connectToServer(
    hostname, username, privateKey string,
    port, poolSize int,
    tlsInsecure bool,
) (client, connection, error)
```

**Prefer:**

```golang
type connectToServerOptions struct {
    hostname, username, privateKey string
    port, poolSize int
    tlsInsecure bool
}

type connectoToServerResult struct {
    client client
    connection connection
}

func connectToServer(opts connectToServerOptions) (connectoToServerResult, error)
```

### Booleans in public and private APIs

Don't use boolean arguments in public APIs. Instead, the public API should have separate functions for
different calls while the boolean parameters should be used in a private functions
or as struct fields to dry-up the code.

**Avoid:**

```golang
func RepeatExecution(wasSuccessful bool) {}
```

**Prefer:**

```golang
func RepeatFailedExecution() {
   repeatExecution(false)
}

func repeatExecution(wasSuccessful bool) {}

func RepeatSuccessfulExecution() {
   repeatExecution(true)
}
```

### Structured logging: Fields casing

Use **snake_case** for structured logging fields.

**Avoid:**

```golang
logger.WithFields(logrus.Fields{"CleanupSTD": "out"})
```

**Prefer:**

```golang
logger.WithFields(logrus.Fields{"cleanup_std": "out"})
```

### Ordering of functions

If a function is being used in the same file, put it right after its first usage.
This allows to easily follow the code without scrolling up and down.

**Avoid:**

```golang
func RepeatFailedExecution() {
   repeatExecution(false)
}

func RepeatSuccessfulExecution() {
   repeatExecution(true)
}

func repeatExecution(wasSuccessful bool) {}
```

**Prefer:**

```golang
func RepeatFailedExecution() {
   repeatExecution(false)
}

func repeatExecution(wasSuccessful bool) {}

func RepeatSuccessfulExecution() {
   repeatExecution(true)
}
```

## Linters

We use [golangci-lint](https://github.com/golangci/golangci-lint) to lint our code against a set of rules.
The rules can be found in the [`.golangci.yml`](http://gitlab.com/gitlab-org/gitlab-runner/blob/master/.golangci.yml) configuration file.

To make sure your code passes the linting rules before pushing it, run `make lint`.

GoLand and VSCode have plugins supporting `golangci-lint` automatically.
