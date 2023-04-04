---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Autoscale configuration (alpha)

GitLab Runner can autoscale so that your infrastructure contains only the required amount of build
instances at any time. You can use autoscaling to leverage your cloud infrastructure to run multiple jobs
simultaneously.

## How autoscaling works

## Autoscaler executors

The following executors support autoscaling for GitLab Runner:

- Docker Autoscaler executor: Use this executor to auto-scale your Docker environment.
- Instance executor: Use this executor if jobs need full access to the host instance, operating
system, and attached devices.

### Supported platforms

| Executor                   | Linux                  | macOS                  | Windows                |
|----------------------------|------------------------|------------------------|------------------------|
| Instance executor          | **{check-circle}** Yes | **{check-circle}** Yes | **{check-circle}** Yes |
| Docker Autoscaler executor | **{check-circle}** Yes | **{dotted-circle}** No | **{check-circle}** Yes |

## Get started with autoscaling

To get started with autoscaling:

1. Prepare the environment for the executor:
   - [Docker Autoscaler executor](../../executors/docker_autoscaler.md)
   - [Instance executor](../../executors/instance.md)
1. Configure the executor to support autoscaling.
