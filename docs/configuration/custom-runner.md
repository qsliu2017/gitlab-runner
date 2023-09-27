---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Custom GitLab Runner with AWS CLI and Amazon ECR Credential Helper **(FREE ALL)**

This guide aims to help you create a custom GitLab Runner Docker image that includes the AWS CLI and Amazon ECR Credential Helper. This is particularly useful for CI/CD pipelines that require interaction with AWS services.

Before continuing, ensure that you've already
[installed Docker](https://docs.docker.com/get-docker/) and
[GitLab Runner](../install/index.md) on the same machine.

## Creating a Custom Docker Image

NOTE:
This section provides the steps to create a custom Docker image that includes AWS CLI and Amazon ECR Credential Helper.

1. **Create a Dockerfile**

    Create a `Dockerfile` with the following content:

    ```Dockerfile
    # Control package versions
    ARG GITLAB_RUNNER_VERSION=v16.4.0
    ARG AWS_CLI_VERSION=2.2.30
    ARG ECR_CREDENTIAL_HELPER_VERSION=v0.5.0

    # AWS CLI and Amazon ECR Credential Helper
    FROM amazonlinux as aws-tools
    RUN yum install -y git make gcc curl unzip
    RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWS_CLI_VERSION}.zip" -o "awscliv2.zip" && \
        unzip awscliv2.zip && \
        ./aws/install -i /usr/local/
    RUN git clone --branch ${ECR_CREDENTIAL_HELPER_VERSION} https://github.com/awslabs/amazon-ecr-credential-helper.git /tmp/ecr-credential-helper && \
        cd /tmp/ecr-credential-helper && \
        make && \
        cp bin/local/docker-credential-ecr-login /usr/local/bin/

    # Final image based on GitLab Runner
    FROM gitlab/gitlab-runner:${GITLAB_RUNNER_VERSION}
    RUN apt-get update && apt-get install -y jq procps curl unzip groff libgcrypt tar gzip less openssh-client
    COPY --from=aws-tools /usr/local/bin/ /usr/local/bin/
    ```

1. **Build the Docker Image**

    Run the following command to build your custom Docker image using `.gitlab-ci.yml`:

    ```shell
    docker build -t custom-gitlab-runner:latest .
    ```

1. **Register the Custom Runner**

    Use the following command to register your custom GitLab Runner:

    ```shell
    gitlab-runner register --docker-image custom-gitlab-runner:latest
    ```

## Example `.gitlab-ci.yml`

Here's an example `.gitlab-ci.yml` file that creates the custom GitLab Runner image:

```yaml
variables:
  DOCKER_DRIVER: overlay2
  IMAGE_NAME: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_NAME
  GITLAB_RUNNER_VERSION: v16.4.0
  AWS_CLI_VERSION: 2.13.21
  ECR_CREDENTIAL_HELPER_VERSION: v0.7.0

stages:
  - build

build-image:
  stage: build
  script:
    - echo "Logging into GitLab Container Registry..."
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - echo "Building Docker image..."
    - docker build --build-arg GITLAB_RUNNER_VERSION=${GITLAB_RUNNER_VERSION} --build-arg AWS_CLI_VERSION=${AWS_CLI_VERSION} --build-arg ECR_CREDENTIAL_HELPER_VERSION=${ECR_CREDENTIAL_HELPER_VERSION} -t ${IMAGE_NAME} .
    - echo "Pushing Docker image to GitLab Container Registry..."
    - docker push ${IMAGE_NAME}
  rules:
    - changes:
        - Dockerfile
```
