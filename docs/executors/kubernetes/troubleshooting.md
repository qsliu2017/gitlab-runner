---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Troubleshooting Kubernetes executor

The following errors are commonly encountered when using the Kubernetes executor.

## `Job failed (system failure): timed out waiting for pod to start`

If the cluster cannot schedule the build pod before the timeout defined by `poll_timeout`, the build pod returns an error. The [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime) should be able to delete it.

To fix this issue, increase the `poll_timeout` value in your `config.toml` file.

## `context deadline exceeded`

The `context deadline exceeded` errors in job logs usually indicate that the Kubernetes API client hit a timeout for a given cluster API request.

Check the [metrics of the `kube-apiserver` cluster component](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/) for any signs of:

- Increased response latencies.
- Error rates for common create or delete operations over pods, secrets, ConfigMaps, and other core (v1) resources.

Logs for timeout-driven errors from the `kube-apiserver` operations may appear as:

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

In some cases, the `kube-apiserver` error response might provide additional details of its sub-components failing (such as the Kubernetes cluster's `etcdserver`):

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

These `kube-apiserver` service failures can occur during the creation of the build pod and also during cleanup attempts after completion:

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

## Connection refused when attempting to communicate with the Kubernetes API

When GitLab Runner makes a request to the Kubernetes API and it fails,
it is likely because
[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)
is overloaded and can't accept or process API requests.

## `Error cleaning up pod` and `Job failed (system failure): prepare environment: waiting for pod running`

The following errors occur when Kubernetes fails to schedule the job pod in a timely manner.
GitLab Runner waits for the pod to be ready, but it fails and then tries to clean up the pod, which can also fail.

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

To troubleshoot, check the Kubernetes primary node and all nodes that run a
[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)
instance. Ensure they have all of the resources needed to manage the target number
of pods that you hope to scale up to on the cluster.

To change the time GitLab Runner waits for a pod to reach its `Ready` status, use the
[`poll_timeout`](#other-configtoml-settings) setting.

To better understand how pods are scheduled or why they might not get scheduled
on time, [read about the Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).

## `request did not complete within requested timeout`

The message `request did not complete within requested timeout` observed during build pod creation indicates that a configured [admission control webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) on the Kubernetes cluster is timing out.

Admission control webhooks are a cluster-level administrative control intercept for all API requests they're scoped for, and can cause failures if they do not execute in time.

Admission control webhooks support filters that can finely control which API requests and namespace sources it intercepts. If the Kubernetes API calls from GitLab Runner do not need to pass through an admission control webhook then you may alter the [webhook's selector/filter configuration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector) to ignore the GitLab Runner namespace, or apply exclusion labels/annotations over the GitLab Runner pod by configuring `podAnnotations` or `podLabels` in the [GitLab Runner Helm Chart `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500).

For example, to avoid [DataDog Admission Controller webhook](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator) from intercepting API requests made by the GitLab Runner manager pod, the following can be added:

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

To list a Kubernetes cluster's admission control webhooks, run:

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

The following forms of logs can be observed when an admission control webhook times out:

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

A failure from an admission control webhook may instead appear as:

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

## `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

If using the `alpine` flavor of the [helper image](../configuration/advanced-configuration.md#helper-image),
there can be [DNS issues](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129) related to Alpine's `musl`'s DNS resolver.

Using the `helper_image_flavor = "ubuntu"` option should resolve this.

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?`

This error can occur when [using Docker-in-Docker](#using-dockerdind) if attempts are made to access the DIND service before it has had time to fully start up. For a more detailed explanation, see [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215).

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443`

This error can happen when [using Docker-in-Docker](#using-dockerdind) if the DIND Maximum Transmission Unit (MTU) is larger than the Kubernetes overlay network. DIND uses a default MTU of 1500, which is too large to route across the default overlay network. The DIND MTU can be changed within the service definition:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows`

When you run your CI/CD job, you might receive an error like the following:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

This issue occurs when you [use node selectors](#specify-the-node-to-execute-builds) to run builds on nodes with different operating systems and architectures.

To fix the issue, configure `nodeSelector` so that the runner manager pod is always scheduled on a Linux node. For example, your [`values.yaml` file](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) should contain the following:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## Build pods are assigned the worker node's IAM role instead of Runner IAM role

This issue happens when the worker node IAM role does not have the permission to assume the correct role. To fix this, add the `sts:AssumeRole` permission to the trust relationship of the worker node's IAM role:

```json
{
    "Effect": "Allow",
    "Principal": {
        "AWS": "arn:aws:iam::<AWS_ACCOUNT_NUMBER>:role/<IAM_ROLE_NAME>"
    },
    "Action": "sts:AssumeRole"
}
```

## `Preparation failed: failed to pull image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

This issue happens if you specified a `pull_policy` in your `.gitlab-ci.yml` but there is no policy configured in the Runner's config file. To fix this, add `allowed_pull_policies` to your config according to [Restrict Docker pull policies](#restrict-docker-pull-policies).

## Background processes cause jobs to hang and timeout

Background processes started during job execution can [prevent the build job from exiting](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880). To avoid this you can:

- Double fork the process. For example, `command_to_run < /dev/null &> /dev/null &`.
- Kill the process before exiting the job script.

## Cache-related `permission denied` errors

Files and folders that are generated in your job have certain UNIX ownerships and permissions.
When your files and folders are archived or extracted, UNIX details are retained.
However, the files and folders may mismatch with the `USER` configurations of
[helper images](../configuration/advanced-configuration.md#helper-image).

If you encounter permission-related errors in the `Creating cache ...` step,
you can:

- As a solution, investigate whether the source data is modified,
  for example in the job script that creates the cached files.
- As a workaround, add matching [chown](https://linux.die.net/man/1/chown) and
  [chmod](https://linux.die.net/man/1/chmod) commands.
  to your [(`before_`/`after_`)`script:` directives](https://docs.gitlab.com/ee/ci/yaml/index.html#default).

## Restrict access to job variables

When using Kubernetes executor, users with access to the Kubernetes cluster can read variables used in the job. By default, job variables are stored in:

- ConfigMap - [There is an ongoing MR to change this](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3751)
- Pod's environment section

To restrict access to job variable data, you should use role-based access control (RBAC) so that only GitLab administrators have access to the namespace used by the GitLab Runner.

If you need other users to access the GitLab Runner namespace, set the following `verbs` to restrict the type of access users have in the GitLab Runner namespace:

- For `pods` and `configmaps`:
  - `get`
  - `watch`
  - `list`
- For `pods/exec` and `pods/attach`, use `create`.

Example RBAC definition for authorized users:

```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: gitlab-runner-authorized-users
rules:
- apiGroups: [""]
  resources: ["configmaps", "pods"]
  verbs: ["get", "watch", "list"]
- apiGroups: [""]
  resources: ["pods/exec", "pods/attach"]
  verbs: ["create"]
```
