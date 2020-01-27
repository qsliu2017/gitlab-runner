FROM registry.access.redhat.com/ubi8/ubi

RUN yum install -y git

COPY gitlab-runner_amd64.rpm /tmp/
COPY checksums /tmp/
RUN rpm -i /tmp/gitlab-runner_amd64.rpm; \
    rm /tmp/gitlab-runner_amd64.rpm && \
    mkdir /etc/gitlab-runner/certs && \
    chmod -R 700 /etc/gitlab-runner
