FROM goboring/golang:1.15.6b5 as boringbuilder

COPY . /runner
WORKDIR /runner
RUN go version
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -tags fips -o /tmp/gitlab-runner .

FROM ubuntu:20.04

ARG TARGETPLATFORM

ENV DEBIAN_FRONTEND=noninteractive
# hadolint ignore=DL3008
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends \
        apt-transport-https \
        ca-certificates \
        curl \
        git \
        wget \
        tzdata \
        openssh-client \
    && rm -rf /var/lib/apt/lists/*

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION

COPY dockerfiles/runner/install-deps /tmp/
COPY --from=boringbuilder /tmp/gitlab-runner /usr/bin/gitlab-runner
RUN chmod +x /usr/bin/gitlab-runner
RUN ln -s /usr/bin/gitlab-runner /usr/bin/gitlab-ci-multi-runner
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" "${GIT_LFS_VERSION}"

COPY dockerfiles/runner/fips/entrypoint /
RUN chmod +x /entrypoint

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
