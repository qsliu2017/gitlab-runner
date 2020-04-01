# Best practices

Below are some guidelines you should follow when you use and administer
GitLab Runner.

## Build Directory

GitLab Runner will clone the repository to a path that exists under a
base path better known as the _Builds Directory_. The default location
of this base directory depends on the executor. For:

- [Kubernetes](../executors/kubernetes.md),
  [Docker](../executors/docker.md) and [Docker
  Machine](../executors/docker_machine.md) executors it will be
  `/builds` inside of the container.
- [Shell](../executors/shell.md) executor it will be `$PWD/builds`.
- [SSH](../executors/ssh.md), [VirtualBox](../executors/virtualbox.md)
  and [Parallels](../executors/parallels.md) executors it will be
  `~/builds` in the home directory of the user configured to handle the
  SSH connection to the target machine.
- [Custom](../executors/custom.md) executor no default is provided and
  it must be explicitly configured, otherwise, the job will fail.

The used _Builds Directory_ may be defined explicitly by the user with the
[`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
setting.

TIP: **Tip:**
You can also specify
[`GIT_CLONE_PATH`](https://docs.gitlab.com/ee/ci/yaml/README.html#custom-build-directories)
if you want to clone to a custom directory, and the guideline below
doesn't apply.

GitLab Runner will use the _Builds Directory_ for all the Jobs that it
will run, but nesting them using a specific pattern
`{builds_dir}/$RUNNER_TOKEN_KEY/$CONCURRENT_ID/$NAMESPACE/$PROJECT_NAME`.
For example: `/builds/2mn-ncv-/0/user/playground`.

GitLab Runner does not stop you from storing things inside of the
_Builds Directory_. For example, you can store tools inside of
`/builds/tools` that can be used during CI execution. We **HIGHLY**
discourage this, you should never store anything inside of the _Builds
Directory_. GitLab Runner should have total control over it and does not
provide stability in such cases. If you have dependencies that are
required for your CI, we recommend installing them in some other
place.

## Graceful shutdown

When the runner is installed on a host and runs local executors it will start additional processes for some operations.
They include downloading/uploading artifacts, handling cache.
All of them are executed as `gitlab-runner` commands. A side effect of this is that using `pkill -QUIT gitlab-runner` or `killall QUIT gitlab-runner` can kill these helper processes as well and fail the operations they are responsible for.
Here are two ways to prevent this:

- Register the runner as a local service e.g. systemd with `SIGQUIT` as the kill signal and use `gitlab-runner stop` or `systemctl stop gitlab-runner.service`. This is the default behavior when installing the `.deb` package:

```conf
; /etc/systemd/system/gitlab-runner.service.d/kill.conf
[Service]
KillSignal=SIGQUIT
TimeoutStopSec=__REDACTED__
```

- Manually kill the process with `kill -SIGQUIT [pid]`. You have to find the pid of the main `gitlab-runner` process.
An easy way for this is by looking at logs. It's printed on startup:

```log
$ gitlab-runner run
Runtime platform                                    arch=amd64 os=linux pid=87858 revision=8d21977e version=12.10.0~beta.82.g8d21977e
```
