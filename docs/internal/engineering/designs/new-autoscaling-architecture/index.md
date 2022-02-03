# New GitLab Runner autoscaling architecture

| - | - |
|---|---|
| Last update            | 2022-02-08 |
| Status                 | Request For Comments |
| Authors                | Arran Walker, Tomasz Maczukin, Adrien Kohlbecker, Grzegorz Bizon |
| Architecture Blueprint | <https://docs.gitlab.com/ee/architecture/blueprints/runner_scaling/index.html> |

On our way towards [new autoscaling solution](https://docs.gitlab.com/ee/architecture/blueprints/runner_scaling/index.html)
for GitLab Runner we want to design a new internal architecture that will
allow autoscale environments for not only the Docker executor (which
our existing Docker Machine executor is doing) but also shell, or rather
Remote Instance executor - to support environments that don't have Docker available
(Mac OS, BSD systems, some Windows versions etc.).

We also want to create a public interface that will allow adding
pluggable integrations for different cloud providers. This should give
our community a way to integrate with clouds that we are not
officially supporting.

## General concept

The very general design is showed on the UML diagram bellow. The design
contains a mix of existing interfaces and entities (like for example the
`Executor` interface), changes that need to be added (the `environmentProvider`
interface and whole autoscaling stack), things that would need to be changed
(behavior of `ExecutorProvider` interface's `Create()` method) or examples of
future benefits (the Remote Instance executor, the VirtualBox/Parallels plugins
for the autoscaling mechanism).

### Diagram

```plantuml
set namespaceSeparator ::

namespace common {
  interface ExecutorProvider
  interface Executor {
    Create() Executor
    Acquire()
    Release()
  }

  ExecutorProvider --> Executor : creates
}

namespace executors {
  class DefaultExecutorProvider implements common::ExecutorProvider

  interface environmentProvider {
    Acquire()
    Get()  Environment
    Release(env Environment)
  }

  interface environment {
    ID() UUID
    OS() string
    Architecture() string
    Protocol() string
    Username() string
    Password() string
    Key() []byte
  }
   
  DefaultExecutorProvider --> environmentProvider : uses
  environmentProvider --> environment : manages
}

namespace docker {
  class Executor implements common::Executor

  class ExecutorProvider extends executors::DefaultExecutorProvider
}

namespace remote_instance {
  class Executor implements common::Executor

  class ExeutorProvider extends executors::DefaultExecutorProvider
}

namespace taskscaler {
  class TaskScaler implements executors::environmentProvider {
     capacityPerInstance
     maxInstances
     maxUseCount

     Acquire(key string) (executors::environment, error)
     Get(key string) *Aquisition
     Release(key string)
     Capacity() Capacity
     ConfigureSchedule(...Schedule)
  }

  class Schedule {
    Schedule []CronSchedule
    MinIdleCount int
    MinIdleTime Duration
    ScaleFactor float64
  }
  
  interface provisioner {
    Shutdown(context.Context)
    Request(n int)
    Instances() []Instance
    Capacity() Capacity
  }

  TaskScaler --> Schedule : uses
  TaskScaler --> provisioner : uses
  TaskScaler --> executors::environment : provides
}

namespace fleeting {
  class Provisioner implements taskscaler::provisioner

  class Instance {
    ID() string
    State() provider.State
    Delete()
    Cause() Cause
    ReadyWait(context.Context) bool
    ConnectInfo(context.Context) (ConnectInfo, error)
    RequestedAt() time.Time
    ProvisionedAt() time.Time
    UpdatedAt() time.Time
    DeletedAt() time.Time
  }

  interface InstanceGroup {
    Init(context.Context, Logger, Settings) (ProviderInfo, error)
    Update(context.Context, func(instance string, state State)) error
    Increase(context.Context, int) (int, error)
    Decrease(context.Context, []string) ([]string, error)
    ConnectInfo(context.Context, string) (ConnectInfo, error)
  }

  class ConnectInfo implements executors::environment {
    OS string
    Arch string

    Protocol string
    Username string
    Password string
    Key []byte
  }

  Provisioner --> Instance : manages
  Instance --> InstanceGroup : uses

  Instance --> ConnectInfo : uses
  InstanceGroup --> ConnectInfo : uses
}

namespace instance_management_plugins {
  namespace GCP {
    class plugin implements fleeting::InstanceGroup
  }

  namespace AWS {
    class plugin implements fleeting::InstanceGroup
  }

  namespace Azure {
    class plugin implements fleeting::InstanceGroup
  }

  namespace Orka {
    class plugin implements fleeting::InstanceGroup
  }

  namespace VirtualBox {
    class plugin implements fleeting::InstanceGroup
  }

  namespace Parallels {
    class plugin implements fleeting::InstanceGroup
  }
}
```

### Description

#### `common::Executor` interface

The interface is what we have right now. This design doesn't focus on changing that interface.
And at least for now it seems that we will not need to do that.

The interface describes the abstraction of Runner's executor - a mechanism that executes
CI/CD job's scripts in specified environment, in a specified way.

#### `common::ExecutorProvider` interface

Defines the abstraction around how different executors are instantiated.

While working on the new autoscaling mechanism we may need to update it a little. At the
moment of writing this document it's still unsure. But even if needed, the changes
would be little.

#### `executors::DefaultExecutorProvider` structure

This is the main implementation of `ExecutorProvider` that we have in Runner. Apart from
Docker Machine executor, every executor is using this provider to create executor instances.

While working on the new autoscaling this is a part that will require some changes. We will
implement a way to force usage of autoscaling environment provider. When defined, the
`ExecutorProvider` will communicate with the autoscaler implementation to request and use
environments for executors to use.

The environment description will be made available for specific implementations to hook
into creation process after the environment description is received but before the executor
instance is created. Executor specific providers will use that environment information to update
the configuration.

Using the new feature - forcing the autoscaling usage - we would register the new providers
on new names:

```golang
common.RegisterExecutorProvider("docker+autoscaling", ...)
common.RegisterExecutorProvider("docker-windows+autoscaling", ...)
common.RegisterExecutorProvider("remote-instance", ...)
```

To use autoscaling the relevant name would need to be used for `[[runners]]`'s `executor` setting.
Provider registered for these names would set the attribute forcing autoscaling to `true`
and `DefaultExecutorAutoscaler` will require the autoscaling configuration to be present in that
case.

#### `docker::Executor` structure

This is the Docker Executor that we have now. Without any changes.

It would support both the Linux and Windows environments, which additionally to bring
Docker Machine's replacement for Linux, would bring Docker Machine's equivalent for Windows
(which currently doesn't exists).

#### `docker::ExecutorProvider` structure

This will be a specific implementation that extends `DefaultExecutorProvider`. It will use the
received SSH/WinRM credentials to connect to the VM. After that, it will:

1. Detect that Docker Engine is working on the `tcp/2376` port.
1. Will download the TLS CA certificate, certificate and key from a well known path.
1. Will use the environment details (IP address) and the downloaded files to fill up
   the Docker Executor's configuration fields.

Making sure that Docker Engine is enabled and listens on `tcp/2376` port and that the
TLS client authentication files are present on specified paths will be responsibility
of the environment creator. Runner will not try to install nor configure the Docker Engine
in the environment.

#### `remote_instance::Executor` structure

This is a new executor that we propose.

It will deprecate and finally replace the existing SSH Executor, bringing support for
autoscaling and WinRM communication protocol in addition to SSH.

It will also make it possible to deprecate and remove VirtualBox and Parallels executors
as separate entities.

As these two are just provisioners for VirtualBox/Parallels VMs that need to expose
specified SSH credentials. With Remote Instance executor we will be able to port the
existing code from VirtualBox/Parallels executors and make it yet another plugins for
the designed autoscaler. It would then provide environment information for Remote Instance
executor to use and just use VirtualBox/Parallels to provision the environments.

Additionally we will be able to start supporting Windows environments within VirtualBox/Parallels
VMs.

#### `remote_instance::ExecutorProvider` structure

This will be a specific implementation that extends `DefaultExecutorProvider`. It will use
the received SSH/WinRM credentials as well as other environment details (like IP address)
to update the configuration of the Remote Instance executor.

It will also check if reported Operating System matches the required configuration. As the
job scripts are created on the Runner host, for now the OS of Runner and the job execution
environment must equal (so that we will not try to use Unix paths on Windows or vice versa).

Depending on the configuration it will also either configure to use the shell reported in the
environment details or validate if it matches configuration provided for the Remove Instance
executor. In the second case, if validation will fail the executor will not be created.

#### `executors::environmentProvider` interface

This interface abstracts out behavior of autoscaling.

In the future we may consider making this interface public and allow people to bring
their own environment provider implementations. But this is beyond current scope.

For now, the interface is added purely to decouple the executor provisioning implementation
from the autoscaling implementation.

#### `executors::environment` interface

This interface abstracts the environment information itself. As with the `EnvironmentProvider`
one it's defined only to allow clear decoupling between executor provisioning and autoscaling
packages.

#### `taskscaler::Taskscaler` structure

This is the implementation of `executors::environmentProvider`. It's the part that handles
autoscaling algorithm.

This mechanism works on **Acquisition** level, where **Acquisition** defies a place to
run a job (the environment that it provides). This may be a whole instance, but doesn't
need to be. Such design allows us to schedule multiple jobs to be executed on the
same VM. But such possibility would be fully optional. The `capacityPerInstance`
field is what enables that behavior. `1` means that the instance is dedicated to only
one job. `2` or more means that multiple jobs can be executed on it.

#### `taskscaler::Schedule` structure

It's a utility structure that holds autoscaling algorithm parameters. `Taskscaler` may
contain multiple schedules. Each `Schedule` may contain multiple `CronSchedules` that
define periods in which such configuration is activated.

Behavior of this goes as follows:

1. When provisioning is going to be initiated (for asking instances creations or removal),
   `Taskscaler` iterates over the slice of defined `Schedule` entries. It checks if any of
   `CronSchedule` periods covers curren time.
1. If it does, the schedule is selected but searching is continued.
1. If another `Schedule` also matches current time - it takes the priority.
1. Finally, the last matching `Schedule` is selected as the acive one.

For that to work we must ensure that each `TaskScaler` instance will be defined with one
default `Schedule` covering any time, added as the last one on the list and containing
default autoscaling parameters.

#### `taskscaler::provisioner` interface

It's an internal interface to decouple the package from the `fleeting` one.

`provisioner` is the part that works at the instance level. It manages instances
basing on the autoscaling configuration and current needs.

#### `fleeting::Provisioner` structure

This is the implementation of `taskscaler::provisioner`'s interface.

#### `fleeting::Instance` structure

It's the structure that represents a single provisioned instance.

#### `fleeting::ConnectInfo` structure

It's a utility structure that provides connection information for a specific instance.

It informs about the detected OS and architecture, available communication protocol
(`ssh` or `winrm`) and credentials.

It also implements `executors::environment` interface, which is the second part of
integrating `taskscaler` and `fleeting` as the `executors::environmentProvider`
implementation.

#### `fleeting::InstanceGroup` interface

This interface brings an integration point to support different cloud providers and
platforms. It's an entrypoint to bring support for different cloud providers, virtualization
hypervisors and possibly other mechanisms that may provide the environment to be used
with Docker or Remote Instance executors.

This interface defines also the boundary at which the public interface will be exposed
for community to contribute.

We may and probably will introduce first few plugins to support most popular clouds.
But this interface is also the place where we will introduce the plugin system, to allow
external integrations to be easily connected to the Runner, without a need for recompiling.

#### `instance_management_plugins`

`GCP`, `AWS`, `Azure`, `Orka`, `VirtualBox`, `Parallels` are examples of instance group
integration plugins that may be added to the Runner with the `fleeting::InstanceGroup`
interface.

## Example configuration

### Example 1: simple Docker executor

```toml
[[runners]]
  name = "docker-static"
  executor = "docker"
  # (...)
  [runners.docker]
    tls_cert_path = "/path/to/certificate_and_keys/directory/"
    host = "some.host:2375"
    image = "alpine:latest"
    # (...)
```

This one uses the Docker Executor as we know now. It uses the static configuration - in this case
pointed to `some.host:2375` with specified TLS certificates path that should contain the CA certificate,
TLS Client Authentication certificate and TLS Client Authentication key. Docker executor will run jobs
in containers on the single Docker Engine pointed by those credentials.

If the `tls_cert_path` and `host` fields would be dropped, then Docker Executor would assume that a locally
available one should be used.

For executor creation the currently registered `docker` provider will be used.

**Our new design doesn't change anything in this scope.**

### Example 2: autoscaled Docker Executor

```toml
[[runners]]
  name = "docker-autoscaled"
  executor = "docker+autoscaling"
  # (...)
  [runners.docker]
    image = "alpine:latest"
    # (...)
    [runners.docker.autoscaling]
      instance_provider = "gcp_plugin" # How this would be configurable depends on
                                       # the discussion happening in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28843
      [runners.docker.autoscaling.instance_provider_config] # content of this section is not used by Runner. It jut tranlates
                                                                 # TOML to JSON and passes to the SetConfig() method of the plugin
        google_project = "project-id"
        autoscaling_group = "id-of-the-group"

      [runners.docker.autoscaling.parameters] # A specific syntax for autoscaling parameters is
                                                                    # something that we will work on further in the design process.
                                                                    # This is just an example
        IdleCount = 10
        IdleScaleFactor = 1
        Limit = 100 # Should we re-use here limit information from the executor like Docker Machine executor does?
        MaxGrowth = 10
        MaxBuilds = 1
        IdleTime = 3600
        [[runners.docker.autoscaled_cloud.autoscaling]]
          # Same concept as we have now
        [[runners.docker.autoscaled_cloud.autoscaling]]
          # Same concept as we have now
```

This one uses the Docker Executor in a way that Docker Machine executor did. Configuration contains
details about autoscaling. `executors::environmentProvider` (so `taskscaler` with the usage of `fleeting`)
will manage VMs. Credentials from these VMs will be dynamically added to Docker Executor's configuration
before creating the executor's instance for a job.

For executor creation a newly registered `docker+autoscaling` provider - which forces the usage
of autoscaling mechanism - will be used.

`docker+autoscaling` provider will be based on the `docker::ExecutorProvider`, which will use environment
information - IP, protocol, protocol credentials - do connect with the VM, check if Docker Engine is available
at the expected TCP port and receive TLS certificates and key for authentication.

**Our new design brings new way of configuration and new autoscaling mechanism to replace the
existing Docker Machine executor.**

### Example 3: simple Windows Docker executor

```toml
[[runners]]
  name = "docker-windows-static"
  executor = "docker-windows"
  # (...)
  [runners.docker]
    tls_cert_path = "c:\\path\\to\\certificate_and_keys\\directory\\"
    host = "some.host:2375"
    image = "alpine:latest"
    # (...)
```

It's the same example as for [simple Docker executor](#example-1-simple-docker-executor), it just uses the
Windows version of Docker executor.

**Our new design doesn't change anything in this scope.**

### Example 4: autoscaled Windows Docker executor

```toml
[[runners]]
  name = "docker-windows-autoscaled"
  executor = "docker-windows+autoscaling"
  # (...)
  [runners.docker]
    image = "alpine:latest"
    # (...)
    [runners.docker.autoscaling]
      instance_provider = "gcp_plugin" # How this would be configurable depends on
                                       # the discussion happening in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28843
      [runners.docker.autoscaling.instance_provider_config] # content of this section is not used by Runner. It jut tranlates
                                                                 # TOML to JSON and passes to the SetConfig() method of the plugin
        google_project = "project-id"
        autoscaling_group = "id-of-the-group"

      [runners.docker.autoscaling.parameters] # A specific syntax for autoscaling parameters is
                                                                    # something that we will work on further in the design process.
                                                                    # This is just an example
        IdleCount = 10
        IdleScaleFactor = 1
        Limit = 100 # Should we re-use here limit information from the executor like Docker Machine executor does?
        MaxGrowth = 10
        MaxBuilds = 1
        IdleTime = 3600
        [[runners.docker.autoscaled_cloud.autoscaling]]
          # Same concept as we have now
        [[runners.docker.autoscaled_cloud.autoscaling]]
          # Same concept as we have now
```

It's the same example as for [autoscaled Docker executor](#example-2-autoscaled-docker-executor), it just uses the
Windows version of Docker executor.

**Our new design makes this a new feature, as such configuration was not possible with Docker Machine
executor. We're basically adding autoscaling possibilities to the Windows Docker executor.**

### Example 5: simple Remote Instance executor

```toml
[[runners]]
  name = "remote-instance-static"
  executor = "remote-instance"
  # (...)
  [runners.remote_instance]
    protocol = "ssh"
    host = "some.host"
    port = 2222
    # (...)
```

It's a new executor, but currently the same configuration could be handled with the use of SSH executor.

It uses statically defined host, port, credentials and protocol. All jobs will be executed in a shell
available at the other side of this SSH/WinRM connection **on a single, statically pointed configuration**.

For executor creation a newly registered `remote-instance` provider will be used.

**Our new design proposes a new Remote Instance executor as a replacement for the existing SSH one.
It would additionally support usage of WinRM protocol for communication with the remote.**

### Example 6: autoscaled Remote Instance executor

```toml
[[runners]]
  name = "remote-instance-autoscaled"
  executor = "remote-instance+autoscaling"
  # (...)
  [runners.remote_instance]
    # (...)
    [runners.remote_instance.autoscaling]
      instance_provider = "digitalocean_plugin" # How this would be configurable depends on
                                                # the discussion happening in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28843
      [runners.remote_instance.autoscaling.instance_provider_config] # content of this section is not used by Runner. It jut tranlates
                                                                 # TOML to JSON and passes to the SetConfig() method of the plugin
        api_token = "some-token"
        size = "s-1vcpu-2gb"
        # (...)

      [runners.remote_instance.autoscaling.parameters] # A specific syntax for autoscaling parameters is
                                                       # something that we will work on further in the design process.
                                                       # This is just an example
        IdleCount = 10
        IdleScaleFactor = 1
        Limit = 100 # Should we re-use here limit information from the executor like Docker Machine executor does?
        MaxGrowth = 10
        MaxBuilds = 1
        IdleTime = 3600
        [[runners.docker.autoscaled_cloud.autoscaling]]
          # Same concept as we have now
        [[runners.docker.autoscaled_cloud.autoscaling]]
          # Same concept as we have now
```

This one also uses the new Remote Instance executor, but configuration is provided dynamically, basing on the
autoscaling parameters.

For executor creation a newly registered `remote-instance+autoscaling` provider - which forces the usage
of autoscaling mechanism - will be used.

`remote-instance+autoscaling` provider will be based on the `remote_instance::ExecutorProvider`, which will
take the environment information - IP, protocol, protocol credentials - and put them into Remote Instance configuration.

As for now we need to match Runner's and job execution's environments when we want to handle jobs on
Windows (paths in scripts will be generated in Runner using local OS path style), this provider should be able
to detect whether provided environment matches such requirement. If either of OSes is Windows, the second one
must match, otherwise provisioning should fail.

**Our new design makes this a new feature. With it we're basically creating an autoscaled Shell
executor, which we didn't have now. This will support systems that don't have Docker (like MacOS, different
BSD versions etc). It also allows us to deprecate and in the future remove SSH, VirtualBox and Parallels
executors.**
