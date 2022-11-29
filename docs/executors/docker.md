---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# The Docker executor **(FREE)**

GitLab Runner uses the Docker executor to run jobs on Docker images.

You can use the Docker executor to:

- Use reproducible build environments that also run on your workstation.
- Test commands from your shell instead of testing them on a dedicated CI/CD server.

The Docker executor uses [Docker Engine](https://www.docker.com/products/container-runtime/)
to run each build in a separate and isolated container.

To connect to Docker Engine, the Docker executor uses:

- The image and services you define in [`.gitlab-ci.yml`](https://docs.gitlab.com/ee/ci/yaml/index.html).
- The configurations you define in [`config.toml`](../commands/index.md#configuration-file).

## Docker executor workflow

The Docker executor divides a job into multiple steps:

1. **Prepare**: Creates and starts the [services](https://docs.gitlab.com/ee/ci/yaml/#services).
1. **Pre-job**: Clones, restores the [cache](https://docs.gitlab.com/ee/ci/yaml/#cache),
   and downloads [artifacts](https://docs.gitlab.com/ee/ci/yaml/#artifacts) from previous
   stages. This job runs on a special Docker image.
1. **Job**: User build. This job runs on your defined Docker image.
1. **Post-job**: Create cache and upload artifacts to GitLab. This job runs on
   a special Docker Image.

The special Docker image is based on [Alpine Linux](https://alpinelinux.org/), and contains the tools
to run the prepare, pre-job, and post-job steps. For the definition of the special Docker image, see
[the GitLab Runner repository](https://gitlab.com/gitlab-org/gitlab-runner/-/tree/v13.4.1/dockerfiles/runner-helper).

## Supported configurations

The Docker executor supports the following configurations:

| GitLab Runner is installed on:   | Docker executor is:     | Container is running on: |
|----------------------------------|-------------------------|--------------------------|
| Windows                          | `docker-windows`        | Windows                  |
| Windows                          | `docker`                | Linux                    |
| Linux                            | `docker`                | Linux                    |

These configurations are **not** supported:

| GitLab Runner is installed on:   | Docker executor is:     | Container is running on: |
|----------------------------------|-------------------------|--------------------------|
| Linux                            | `docker-windows`        | Linux                    |
| Linux                            | `docker`                | Windows                  |
| Linux                            | `docker-windows`        | Windows                  |
| Windows                          | `docker`                | Windows                  |
| Windows                          | `docker-windows`        | Linux                    |

NOTE:
GitLab Runner uses Docker Engine API
[v1.25](https://docs.docker.com/engine/api/v1.25/) to communicate with the Docker
Engine. This means that the [minimum supported version](https://docs.docker.com/engine/api/#api-version-matrix)
of Docker on a Linux server is `1.13.0`. The supported Windows Server needs to be more [recent](#supported-docker-versions).

### Supported shells

To run your build with the `image` directive on your Docker image, you must
have a working shell with its operating system `PATH`.

GitLab Runner supports the following shells:

- Windows
  - PowerShell
- Linux
  - `sh`
  - `bash`
  - `pwsh`. [Available in GitLab Runner 13.9 and later](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4021).

GitLab Runner cannot execute a command using the underlying OS system calls, such as `exec`.

## Docker executor network configurations

You must configure a network to connect services to a CI/CD job. You can also configure a network to run jobs in user-defined networks.

To configure a network, you can either:

- Configure a runner to create a network per build. Recommended.
- Define container links. Container links are a legacy feature of Docker.

### Configure the runner to create a network per job

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1042) in GitLab Runner 12.9.

- Prerequisites:
  - You must enable the [`FF_NETWORK_PER_BUILD` feature flag](../configuration/feature-flags.md)

To configure the runner to create a bridge network for each job, set the `FF_NETWORK_PER_BUILD` variable
in `config.toml`.

To set the default Docker address pool, use `default-address-pool` in
[`dockerd`](https://docs.docker.com/engine/reference/commandline/dockerd/). If CIDR ranges are already
in use, Docker networks might conflict with other networks on the host, including other Docker networks.

To enable IPv6 support for this network, set `enable_ipv6` to `true` in the Docker configuration.
This feature works only when the Docker daemon is configured with IPv6 enabled. To enable IPv6 support on your host, see the [Docker documentation](https://docs.docker.com/config/daemon/ipv6/).

The job container is resolvable by using the `build` alias as well, because the hostname is assigned by GitLab.

#### How network per job works

When a job starts, the runner creates a bridge network similar to `docker network create <network>`.
After the bridge network is created, it connects to the service and build job containers.
Unlike [container links](#configure-a-network-with-container-links),
Docker environment variables are not shared across the containers.

The container running the job and the containers running the service
resolve each other's hostnames and aliases. This functionality is
[provided by Docker](https://docs.docker.com/network/bridge/#differences-between-user-defined-bridges-and-the-default-bridge).

The runner removes the network at the end of the job.

### Configure a network with container links

The default network mode uses Docker [legacy container links](https://docs.docker.com/network/links/) and the
the default Docker `bridge` to link the job container with the services.

To configure the network, in the `config.toml` file, in the `[runners.docker]` section, update the `network_mode`. Use
one for the following Docker [networking mode](https://docs.docker.com/engine/reference/run/#network-settings) values:

- `bridge`: Use the bridge network. Default.
- `host`: Use the host's network stack inside the container.
- `none`: No networking. Not recommended.

```toml
[runners.docker]
  network_mode = "bridge"
```

If you use any other `network_mode` value, these are taken as the name of an already existing
Docker network, which the build container should connect to.

For name resolution to work, Docker updates the `/etc/hosts` file in the
container with the service container hostname and alias. However,
the service container is **not** able to resolve the container
name. To resolve the container name, you must create a network for each job.

Linked containers share their environment variables.

## Use the Docker executor

To use the Docker executor, in `config.toml`, define Docker as the executor:

```toml
[[runners]]
  executor = "docker"
```

## Define Docker images and services in `.gitlab-ci.yml`

To configure the Docker executor, you define the Docker images and services in `.gitlab-ci.yml`.
If you don't define an `image` in `.gitlab-ci.yml`, the executor uses the `image` defined in `config.toml`.

Use the following keywords:

- `image`: The name of the Docker image that the executor uses to run jobs.
  - You can enter an image from the local Docker Engine, or any image in
   Docker Hub. For more information, see the [Docker documentation](https://docs.docker.com/get-started/overview/).
  - To define the image version, use a colon (`:`) to add a tag. If you don't specify a tag,
   Docker implies `latest` as the version.
- `services`: The additional image that creates another container and links to the `image`. For more information about types of services, see [Services](https://docs.gitlab.com/ee/ci/services/).

Example:

```yaml
image: ruby:2.7

services:
  - postgres:9.3

before_script:
  - bundle install

test:
  script:
  - bundle exec rake spec
```

To define different images and services per job:

```yaml
before_script:
  - bundle install

test:2.6:
  image: ruby:2.6
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:2.7:
  image: ruby:2.7
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

If you don't specify the namespace, Docker implies `library`, which includes all
[official images](https://hub.docker.com/u/library/). This means that you can omit
`library` in `.gitlab-ci.yml` and `config.toml`. For example, you can define an image like
`image: ruby:2.7`, which is a shortcut for `image: library/ruby:2.7`.

### Define an image from a private registry

Prerequisites:

- To access images from a private registry, you must [authenticate GitLab Runner](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#access-an-image-from-a-private-container-registry).

To define images from private registries, provide the registry name and the image in `.gitlab-ci.yml`.

Example:

```yaml
image: my.registry.tld:5000/namepace/image:tag
```

In this example, GitLab Runner searches the registry `my.registry.tld:5000` for the
image `namespace/image:tag`.

## Define images and services in `config.toml`

To add images and services to all builds run by a runner, update `[runners.docker]` in the `config.toml`.

Example:

```toml
[runners.docker]
  image = "ruby:2.7"

[[runners.docker.services]]
  name = "mysql:latest"
  alias = "db"

[[runners.docker.services]]
  name = "redis:latest"
  alias = "cache"
```

## Restrict Docker images and services

You can restrict the Docker images that can run your jobs.
To do this, you specify wildcard patterns. For example, to allow images
from your private Docker registry only:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/*:*"]
    allowed_services = ["my.registry.tld:5000/*:*"]
```

Or, to restrict to a specific list of images from this registry:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/ruby:*", "my.registry.tld:5000/node:*"]
    allowed_services = ["postgres:9.4", "postgres:latest"]
```

## Restrict Docker pull policies

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26753) in GitLab 15.1.

In the `.gitlab-ci.yml` file, you can specify a pull policy. This policy determines how
a CI/CD job should fetch images.

To restrict which pull policies can be used in the `.gitlab-ci.yml` file, you can use `allowed_pull_policies`.

For example, to allow only the `always` and `if-not-present` pull policies:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- If you don't specify `allowed_pull_policies`, the default is the value in the `pull_policy` keyword.
- If you don't specify `pull_policy`, the default is `always`.
- The existing [`pull_policy` keyword](../executors/docker.md#how-pull-policies-work) must not include a pull policy
  that is not specified in `allowed_pull_policies`. If it does, the job returns an error.

## Accessing the services

Let's say that you need a Wordpress instance to test some API integration with
your application.

You can then use for example the [tutum/wordpress](https://hub.docker.com/r/tutum/wordpress/) as a service image in your
`.gitlab-ci.yml`:

```yaml
services:
- tutum/wordpress:latest
```

When the build is run, `tutum/wordpress` will be started first and you will have
access to it from your build container under the hostname `tutum__wordpress`
and `tutum-wordpress`.

The GitLab Runner creates two alias hostnames for the service that you can use
alternatively. The aliases are taken from the image name following these rules:

1. Everything after `:` is stripped.
1. For the first alias, the slash (`/`) is replaced with double underscores (`__`).
1. For the second alias, the slash (`/`) is replaced with a single dash (`-`).

Using a private service image will strip any port given and apply the rules as
described above. A service `registry.gitlab-wp.com:4999/tutum/wordpress` will
result in hostname `registry.gitlab-wp.com__tutum__wordpress` and
`registry.gitlab-wp.com-tutum-wordpress`.

## Configuring services

Many services accept environment variables which allow you to easily change
database names or set account names depending on the environment.

GitLab Runner 0.5.0 and up passes all YAML-defined variables to the created
service containers.

For all possible configuration variables check the documentation of each image
provided in their corresponding Docker Hub page.

All variables are passed to all services containers. It's not designed to
distinguish which variable should go where.
Secure variables are only passed to the build container.

## Mounting a directory in RAM

You can mount a path in RAM using tmpfs. This can speed up the time required to test if there is a lot of I/O related work, such as with databases.
If you use the `tmpfs` and `services_tmpfs` options in the runner configuration, you can specify multiple paths, each with its own options. See the [Docker reference](https://docs.docker.com/engine/reference/commandline/run/#mount-tmpfs-tmpfs) for details.
This is an example `config.toml` to mount the data directory for the official Mysql container in RAM.

```toml
[runners.docker]
  # For the main container
  [runners.docker.tmpfs]
      "/var/lib/mysql" = "rw,noexec"

  # For services
  [runners.docker.services_tmpfs]
      "/var/lib/mysql" = "rw,noexec"
```

## Specify Docker driver operations

Specify arguments to supply to the Docker volume driver when you create volumes for builds.
For example, you can use these arguments to limit the space for each build to run, in addition to all other driver specific options.
The following example shows a `config.toml` where the limit that each build can consume is set to 50GB.

```toml
[runners.docker]
  [runners.docker.volume_driver_ops]
      "size" = "50G"
```

## Build directory in service

Since version 1.5 GitLab Runner mounts a `/builds` directory to all shared services.

See an issue: <https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1520>.

### PostgreSQL service example

See the specific documentation for
[using PostgreSQL as a service](https://docs.gitlab.com/ee/ci/services/postgres.html).

### MySQL service example

See the specific documentation for
[using MySQL as a service](https://docs.gitlab.com/ee/ci/services/mysql.html).

### The services health check

After the service is started, GitLab Runner waits some time for the service to
be responsive. Currently, the Docker executor tries to open a TCP connection to
the first exposed service in the service container.

You can see how it is implemented by checking this [Go command](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/commands/helpers/health_check.go).

## The builds and cache storage

The Docker executor by default stores all builds in
`/builds/<namespace>/<project-name>` and all caches in `/cache` (inside the
container).
You can overwrite the `/builds` and `/cache` directories by defining the
`builds_dir` and `cache_dir` options under the `[[runners]]` section in
`config.toml`. This will modify where the data are stored inside the container.

If you modify the `/cache` storage path, you also need to make sure to mark this
directory as persistent by defining it in `volumes = ["/my/cache/"]` under the
`[runners.docker]` section in `config.toml`.

### Clearing Docker cache

> Introduced in GitLab Runner 13.9, [all created runner resources cleaned up](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2310).

GitLab Runner provides the [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache)
script to remove old containers and volumes that can unnecessarily consume disk space.

Run `clear-docker-cache` regularly (using `cron` once per week, for example),
ensuring a balance is struck between:

- Maintaining some recent containers in the cache for performance.
- Reclaiming disk space.

`clear-docker-cache` can remove old or unused containers and volumes that are created by the GitLab Runner. For a list of options, run the script with `help` option:

```shell
clear-docker-cache help
```

The default option is `prune-volumes` which the script will remove all unused containers (both dangling and unreferenced) and volumes.

### Clearing old build images

The [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache) script will not remove the Docker images as they are not tagged by the GitLab Runner. You can however confirm the space that can be reclaimed by running the script with the `space` option as illustrated below:

```shell
clear-docker-cache space

Show docker disk usage
----------------------

TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
Images          14        9         1.306GB   545.8MB (41%)
Containers      19        18        115kB     0B (0%)
Local Volumes   0         0         0B        0B
Build Cache     0         0         0B        0B
```

Once you have confirmed the reclaimable space, run the [`docker system prune`](https://docs.docker.com/engine/reference/commandline/system_prune/) command that will remove all unused containers, networks, images (both dangling and unreferenced), and optionally, volumes that are not tagged by the GitLab Runner.

## The persistent storage

The Docker executor can provide a persistent storage when running the containers.
All directories defined under `volumes =` will be persistent between builds.

The `volumes` directive supports two types of storage:

1. `<path>` - **the dynamic storage**. The `<path>` is persistent between subsequent
   runs of the same concurrent job for that project. The data is attached to a
   custom cache volume: `runner-<short-token>-project-<id>-concurrent-<concurrency-id>-cache-<md5-of-path>`.
1. `<host-path>:<path>[:<mode>]` - **the host-bound storage**. The `<path>` is
   bound to `<host-path>` on the host system. The optional `<mode>` can specify
   that this storage is read-only or read-write (default).

### The persistent storage for builds

If you make the `/builds` directory **a host-bound storage**, your builds will be stored in:
`/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`, where:

- `<short-token>` is a shortened version of the Runner's token (first 8 letters)
- `<concurrent-id>` is a unique number, identifying the local job ID on the
  particular runner in context of the project

## The privileged mode

The Docker executor supports a number of options that allows fine-tuning of the
build container. One of these options is the [`privileged` mode](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities).

### Use Docker-in-Docker with privileged mode

The configured `privileged` flag is passed to the build container and all
services, thus allowing to easily use the Docker-in-Docker approach.

First, configure your runner (`config.toml`) to run in `privileged` mode:

```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    privileged = true
```

Then, make your build script (`.gitlab-ci.yml`) to use Docker-in-Docker
container:

```yaml
image: docker:git
services:
- docker:dind

build:
  script:
  - docker build -t my-image .
  - docker push my-image
```

## The ENTRYPOINT

The Docker executor doesn't overwrite the [`ENTRYPOINT` of a Docker image](https://docs.docker.com/engine/reference/run/#entrypoint-default-command-to-execute-at-runtime).

That means that if your image defines the `ENTRYPOINT` and doesn't allow running
scripts with `CMD`, the image will not work with the Docker executor.

With the use of `ENTRYPOINT` it is possible to create special Docker image that
would run the build script in a custom environment, or in secure mode.

You may think of creating a Docker image that uses an `ENTRYPOINT` that doesn't
execute the build script, but does execute a predefined set of commands, for
example to build the Docker image from your directory. In that case, you can
run the build container in [privileged mode](#the-privileged-mode), and make
the build environment of the runner secure.

Consider the following example:

1. Create a new Dockerfile:

   ```dockerfile
   FROM docker:dind
   ADD / /entrypoint.sh
   ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
   ```

1. Create a bash script (`entrypoint.sh`) that will be used as the `ENTRYPOINT`:

   ```shell
   #!/bin/sh

   dind docker daemon
       --host=unix:///var/run/docker.sock \
       --host=tcp://0.0.0.0:2375 \
       --storage-driver=vf &

   docker build -t "$BUILD_IMAGE" .
   docker push "$BUILD_IMAGE"
   ```

1. Push the image to the Docker registry.

1. Run Docker executor in `privileged` mode. In `config.toml` define:

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       privileged = true
   ```

1. In your project use the following `.gitlab-ci.yml`:

   ```yaml
   variables:
     BUILD_IMAGE: my.image
   build:
     image: my/docker-build:image
     script:
     - Dummy Script
   ```

This is just one of the examples. With this approach the possibilities are
limitless.

## Use Podman to run Docker commands

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27119) in GitLab 15.3.

If you have GitLab Runner installed on Linux, your jobs can use Podman to replace Docker as the container runtime in the Docker executor.

Prerequisites:

- [Podman](https://podman.io/) v4.2.0 or later.
- To run [services](#services) with Podman as an executor, enable the
  [`FF_NETWORK_PER_BUILD` feature flag](#configure-the-runner-to-create-a-network-per-job).
  [Docker container links](https://docs.docker.com/network/links/) are legacy
  and are not supported by [Podman](https://podman.io/).

1. On your Linux host, install GitLab Runner. If you installed GitLab Runner
   by using your system's package manager, it automatically creates a `gitlab-runner` user.
1. Sign in as the user that will run GitLab Runner. You must do so in a way that
   doesn't go around [`pam_systemd`](https://www.freedesktop.org/software/systemd/man/pam_systemd.html).
   You can use SSH with the correct user. This ensures you can run `systemctl` as this user.
1. Make sure that your system fulfills the prerequisites for
   [a rootless Podman setup](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md).
   Specifically, make sure your user has
   [correct entries in `/etc/subuid` and `/etc/subgid`](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md#etcsubuid-and-etcsubgid-configuration).
1. On the Linux host, [install Podman](https://podman.io/getting-started/installation).
1. Enable and start the Podman socket:

   ```shell
   systemctl --user --now enable podman.socket
   ```

1. Verify the Podman socket is listening:

   ```shell
   systemctl status --user podman.socket
   ```

1. Copy the socket string in the `Listen` key through which Podman's API is being accessed.
1. Edit the GitLab Runner `config.toml` file and add the socket value to the host entry in the `[[runners.docker]]` section.
   For example:

   ```toml
   [[runners]]
     name = "podman-test-runner-2022-06-07"
     url = "https://gitlab.com"
     token = "x-XxXXXXX-xxXxXxxxxx"
     executor = "docker"
     [runners.docker]
       host = "unix:///run/user/1012/podman/podman.sock"
       tls_verify = false
       image = "quay.io/podman/stable"
       privileged = true
   ```

### Using Podman to build container images from a Dockerfile

The example below illustrates how to use Podman to build a container image and push the image to the GitLab Container registry. The default container image in the Runner `config.toml` is set to `quay.io/podman/stable`, which means the CI job will default to using that image to execute the included commands.

```yaml
variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - podman login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - podman build -t $IMAGE_TAG .
    - podman push $IMAGE_TAG
  when: manual
```

### Using Buildah to build container images from a Dockerfile

The example below illustrates how to use [Buildah](https://buildah.io/) to build a container image and push the image to the GitLab Container registry.

```yaml
image: quay.io/buildah/stable

variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - buildah login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - buildah bud -t $IMAGE_TAG .
    - buildah push $IMAGE_TAG
  when: manual
```

## Specify which user runs the job

By default, the runner runs jobs as the `root` user within the container. To specify a different, non-root user to run the job, use the `USER` directive in the Dockerfile of the Docker image.

```dockerfile
FROM amazonlinux
RUN ["yum", "install", "-y", "nginx"]
RUN ["useradd", "www"]
USER "www"
CMD ["/bin/bash"]
```

When you use that Docker image to execute your job, it runs as the specified user:

```yaml
build:
  image: my/docker-build:image
  script:
  - whoami   # www
```

## How pull policies work

When using the `docker` or `docker+machine` executors, you can set the
`pull_policy` parameter in the runner `config.toml` file as described in the configuration docs'
[Docker section](../configuration/advanced-configuration.md#the-runnersdocker-section).

This parameter defines how the runner works when pulling Docker images (for both `image` and `services` keywords).
You can set it to a single value, or a list of pull policies, which will be attempted in order
until an image is pulled successfully.

If you don't set any value for the `pull_policy` parameter, then
the runner will use the `always` pull policy as the default value.

Now let's see how these policies work.

### Using the `never` pull policy

The `never` pull policy disables images pulling completely. If you set the
`pull_policy` parameter of a runner to `never`, then users will be able
to use only the images that have been manually pulled on the Docker host
the runner runs on.

If an image cannot be found locally, then the runner will fail the build
with an error similar to:

```plaintext
Pulling docker image local_image:latest ...
ERROR: Build failed: Error: image local_image:latest not found
```

#### When to use the `never` pull policy

The `never` pull policy should be used if you want or need to have a full
control on which images are used by the runner's users. It is a good choice
for private runners that are dedicated to a project where only specific images
can be used (not publicly available on any registries).

#### When not to use the `never` pull policy

The `never` pull policy will not work properly with most of [auto-scaled](../configuration/autoscale.md)
Docker executor use cases. Because of how auto-scaling works, the `never`
pull policy may be usable only when using a pre-defined cloud instance
images for chosen cloud provider. The image needs to contain installed
Docker Engine and local copy of used images.

### Using the `if-not-present` pull policy

When the `if-not-present` pull policy is used, the runner will first check
if the image is present locally. If it is, then the local version of
image will be used. Otherwise, the runner will try to pull the image.

#### When to use the `if-not-present` pull policy

The `if-not-present` pull policy is a good choice if you want to use images pulled from
remote registries, but you want to reduce time spent on analyzing image
layers difference when using heavy and rarely updated images.
In that case, you will need once in a while to manually remove the image
from the local Docker Engine store to force the update of the image.

It is also the good choice if you need to use images that are built
and available only locally, but on the other hand, also need to allow to
pull images from remote registries.

#### When not to use the `if-not-present` pull policy

The `if-not-present` pull policy should not be used if your builds use images that
are updated frequently and need to be used in most recent versions.
In such a situation, the network load reduction created by this policy may
be less worthy than the necessity of the very frequent deletion of local
copies of images.

This pull policy should also not be used if your runner can be used by
different users which should not have access to private images used
by each other. Especially do not use this pull policy for shared runners.

To understand why the `if-not-present` pull policy creates security issues
when used with private images, read the
[security considerations documentation](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).

### Using the `always` pull policy

The `always` pull policy will ensure that the image is **always** pulled.
When `always` is used, the runner will try to pull the image even if a local
copy is available. The [caching semantics](https://kubernetes.io/docs/concepts/configuration/overview/#container-images)
of the underlying image provider make this policy efficient.
The pull attempt is fast because all image layers are cached.

If the image is not found, then the build will fail with an error similar to:

```plaintext
Pulling docker image registry.tld/my/image:latest ...
ERROR: Build failed: Error: image registry.tld/my/image:latest not found
```

When using the `always` pull policy in GitLab Runner versions older than `v1.8`, it could
fall back to the local copy of an image and print a warning:

```plaintext
Pulling docker image registry.tld/my/image:latest ...
WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found
WARNING: Locally found image will be used instead.
```

This was [changed in GitLab Runner `v1.8`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1905).

#### When to use the `always` pull policy

The `always` pull policy should be used if your runner is publicly available
and configured as a shared runner in your GitLab instance. It is the
only pull policy that can be considered as secure when the runner will
be used with private images.

This is also a good choice if you want to force users to always use
the newest images.

Also, this will be the best solution for an [auto-scaled](../configuration/autoscale.md)
configuration of the runner.

#### When not to use the `always` pull policy

The `always` pull policy will definitely not work if you need to use locally
stored images. In this case, the runner will skip the local copy of the image
and try to pull it from the remote registry. If the image was built locally
and doesn't exist in any public registry (and especially in the default
Docker registry), the build will fail with:

```plaintext
Pulling docker image local_image:latest ...
ERROR: Build failed: Error: image local_image:latest not found
```

### Using multiple pull policies

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26558) in GitLab Runner 13.8.

The `pull_policy` parameter allows you to specify a list of pull policies.
The policies in the list will be attempted in order from left to right until a pull attempt
is successful, or the list is exhausted.

This functionality can be useful when the Docker registry is not available
and you need to increase job resiliency.
If you use the `always` policy and the registry is not available, the job fails even if the desired image is cached locally.

To overcome that behavior, you can add additional fallback pull policies
that execute in case of failure.
By adding a second pull policy value of `if-not-present`, the runner finds any locally-cached Docker image layers:

```toml
[runners.docker]
  pull_policy = ["always", "if-not-present"]
```

**Any** failure to fetch the Docker image causes the runner to attempt the following pull policy.
Examples include an `HTTP 403 Forbidden` or an `HTTP 500 Internal Server Error` response from the repository.

Note that the security implications mentioned in the `When not to use this pull policy?` sub-section of the
[Using the if-not-present pull policy](#using-the-if-not-present-pull-policy) section still apply,
so you should be aware of the security implications and read the
[security considerations documentation](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).

```plaintext
Using Docker executor with image alpine:latest ...
Pulling docker image alpine:latest ...
WARNING: Failed to pull image with policy "always": Error response from daemon: received unexpected HTTP status: 502 Bad Gateway (docker.go:143:0s)
Attempt #2: Trying "if-not-present" pull policy
Using locally found image version due to "if-not-present" pull policy
```

## Retrying a failed pull

You can specify the same policy again to configure a runner
to retry a failed Docker pull.

This is similar to [the `retry` directive](https://docs.gitlab.com/ee/ci/yaml/#retry)
in the `.gitlab-ci.yml` files of individual projects,
but only takes effect if specifically the Docker pull fails initially.

For example, this configuration retries the pull one time:

```toml
[runners.docker]
  pull_policy = ["always", "always"]
```

## Docker vs Docker-SSH (and Docker+Machine vs Docker-SSH+Machine)

WARNING:
Starting with GitLab Runner 10.0, both Docker-SSH and Docker-SSH+machine executors
are **deprecated** and will be removed in one of the upcoming releases.

We provided a support for a special type of Docker executor, namely Docker-SSH
(and the autoscaled version: Docker-SSH+Machine). Docker-SSH uses the same logic
as the Docker executor, but instead of executing the script directly, it uses an
SSH client to connect to the build container.

Docker-SSH then connects to the SSH server that is running inside the container
using its internal IP.

This executor is no longer maintained and will be removed in the near future.

## Using Windows containers with the Docker executor

> [Introduced](https://gitlab.com/groups/gitlab-org/-/epics/535) in GitLab Runner 11.11.

The Docker executor does not support:

- Interactive web terminal.
- Host device mounting.
- Docker-in-Docker because Docker [do not support it]((https://github.com/docker-library/docker/issues/49)).
- Linux containers on Windows because they are experimental. For more information, see [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4373).

### Windows versions supported by GitLab Runner

GitLab Runner only supports the following versions of Windows which
follows our [support lifecycle for Windows](../install/windows.md#windows-version-support-policy):

- Windows Server 21H1/LTSC2022.
- Windows Server 20H2.
- Windows Server 2004.
- Windows Server 1809.

For future Windows Server versions, GitLab has a
[future version support policy](../install/windows.md#windows-version-support-policy).

You can only run containers based on the same OS version that the Docker
daemon is running on. For example, the following [`Windows Server Core`](https://hub.docker.com/_/microsoft-windows-servercore) images can
be used:

- `mcr.microsoft.com/windows/servercore:ltsc2022`
- `mcr.microsoft.com/windows/servercore:ltsc2022-amd64`
- `mcr.microsoft.com/windows/servercore:20H2`
- `mcr.microsoft.com/windows/servercore:20H2-amd64`
- `mcr.microsoft.com/windows/servercore:2004`
- `mcr.microsoft.com/windows/servercore:2004-amd64`
- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`

### Supported Docker versions

Windows Server must use a recent version of Docker so that GitLab Runner can use Docker to
check the Windows Server version.

GitLab Runner does not support Docker v17.06 because it cannot identify the Windows Server version and causes the following error:

```plaintext
unsupported Windows Version: Windows Server Datacenter
```

For troubleshooting information about this error, see [Docker executor: `unsupported Windows Version`](../install/windows.md#docker-executor-unsupported-windows-version).

### Nanoserver support

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2492) in GitLab Runner 13.6.

With Powershell Core support in the Windows helper image, you can leverage `nanoserver` variants for the helper image.

### Known issues for Docker executor on Windows

The following known issues apply to Windows containers with
Docker executor:

- The volume directory must exist when it is mounted, otherwise Docker fails to start the container. For more information, see
  [#3754](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3754).
- `docker-windows` executor can be run only when GitLab Runner is running on Windows.
- If the destination path drive letter is not `c:`, paths are not supported for:

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  This means values like `f:\\cache_dir` are not supported, but `f:` is supported.
  If the destination path is on the `c:` drive, paths are also supported. For example, `c:\\cache_dir`.
  This is due to a [limitation in Docker](https://github.com/MicrosoftDocs/Virtualization-Documentation/issues/334).

### Configure a Docker executor in a Windows container

To configure a Docker executor in a Windows container, use the following configuration example.
For more advanced configuration options for the Docker executor, see the
[advanced configuration](../configuration/advanced-configuration.md#the-runnersdocker-section)
section.

```toml
[[runners]]
  name = "windows-docker-2019"
  url = "https://gitlab.com/"
  token = "xxxxxxx"
  executor = "docker-windows"
  [runners.docker]
    image = "mcr.microsoft.com/windows/servercore:1809_amd64"
    volumes = ["c:\\cache"]
```

NOTE:
When a runner is registered with `c:\\cache`
as a source directory when passing the `--docker-volumes` or
`DOCKER_VOLUMES` environment variable, there is a
[known issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4312).

### Services

In [GitLab Runner 12.9 and later](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1042),
you can use [services](https://docs.gitlab.com/ee/ci/services/) by
enabling [a network for each job](#configure-the-runner-to-create-a-network-per-job).
