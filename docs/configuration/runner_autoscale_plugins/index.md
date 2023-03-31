---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Autoscale configuration (alpha)

GitLab Runner can autoscale so that your infrastructure contains only the required amount of build
instances at any time. You can use autoscaling to leverage your cloud infrastructure to run multiple jobs
simultaneously.

When GitLab Runner is configured to autoscale, a type of runner called a _runner manager_ manages
multiple runners to...

## Autoscaler executors

To autoscale GitLab Runner, you use one of the following executors:

- Docker Autoscaler executor: Use this executor to auto-scale your Docker environment.
- Instance executor: Use this executor if jobs need full access to the host instance, operating
system, and attached devices.

### Supported platforms

| Executor                   | Linux                  | macOS                  | Windows                |
|----------------------------|------------------------|------------------------|------------------------|
| Instance executor          | **{check-circle}** Yes | **{check-circle}** Yes | **{check-circle}** Yes |
| Docker Autoscaler executor | **{check-circle}** Yes | **{dotted-circle}** No | **{check-circle}** Yes |

## Set up autoscaling

To set up autoscaling, you must:

1. Prepare your environment.
1. Configure an executor that supports autoscaling.

### Step 1: Prepare the environment

To prepare your environment for autoscaling, you must install an AWS fleeting plugin. The fleeting plugin
targets the platform that you want to autoscale on.

The AWS fleeting plugin is in alpha. Support for Google Cloud Platform and Azure fleeting plugins
are proposed in <this-issue>.

To install the AWS plugin:

1. [Download the binary](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases) for your host platform.
1. Ensure that the plugin binaries are discoverable through the PATH environment variable.

### Step 2: Select and configure an autoscaling executor

Select and configure the executor:

- Docker autoscaler executor
- Instance executor
