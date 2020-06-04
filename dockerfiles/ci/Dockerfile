FROM golang:1.13.8 as build

ARG DOCKER_VERSION
ARG BUILDX_VERSION
ARG GIT_LFS_VERSION
ARG GIT_LFS_256_CHECKSUM

# Download docker client
RUN mkdir -p /build/usr/bin && \
    wget -q https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz \
         -O /tmp/docker.tar.gz && \
    tar -xzf /tmp/docker.tar.gz -C /tmp/ && \
    mv /tmp/docker/docker* /build/usr/bin && \
    chmod +x /build/usr/bin/docker*

# Download docker buildx plugin
RUN mkdir -p /build/root/.docker/cli-plugins && \
    wget https://github.com/docker/buildx/releases/download/v${BUILDX_VERSION}/buildx-v${BUILDX_VERSION}.linux-amd64 \
         -q -O /build/root/.docker/cli-plugins/docker-buildx && \
    chmod a+x /build/root/.docker/cli-plugins/docker-buildx

# Download git-lfs
COPY dockerfiles/ci/download_git_lfs /tmp/
RUN /tmp/download_git_lfs && \
    chmod 700 /build/root

ARG KUBECTL_VERSION

# Install kubectl
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl \
        -O /build/usr/local/bin/kubectl && \
    chmod +x /build/usr/local/bin/kubectl

FROM golang:1.13.8

ENV LANG=en_US.UTF-8 \
    LANGUAGE=en_US:en \
    LC_ALL=en_US.UTF-8

COPY Makefile* /tmp/build/
COPY dockerfiles/ci/install-deps /tmp/
COPY --from=build --chown=0:0 /build/ /
WORKDIR /tmp

ARG PWSH_VERSION

RUN ./install-deps && rm -rf /tmp/*
