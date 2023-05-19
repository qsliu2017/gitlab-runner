ARG BASE_IMAGE

FROM $BASE_IMAGE

ARG TARGETPLATFORM

# hadolint ignore=DL3008
RUN microdnf update -y && \
    microdnf install -y \
        openssl \
        openssl-devel \
        curl \
        curl-devel \
        wget \
        openssh-clients \
        hostname \
        procps-ng \
        tar \
        gzip \
        expat-devel \
        zlib-devel \
        perl-CPAN \
        perl-devel \
        autoconf \
        gcc \
        which \
        python3 \
        gettext \
        diffutils \
        && microdnf clean all && \
        rm -rf /var/cache/yum

ARG DOCKER_MACHINE_VERSION
ARG DUMB_INIT_VERSION
ARG GIT_LFS_VERSION
ARG GIT_VERSION

COPY gitlab-runner_*.rpm checksums-* install-deps install-gitlab-runner /tmp/
RUN /tmp/install-deps "${TARGETPLATFORM}" "${DOCKER_MACHINE_VERSION}" "${DUMB_INIT_VERSION}" "${GIT_LFS_VERSION}"
RUN rm -rf /tmp/* /etc/gitlab-runner/.runner_system_id

RUN wget https://github.com/git/git/archive/refs/tags/v${GIT_VERSION}.tar.gz && \
    tar xf v${GIT_VERSION}.tar.gz && \
    cd git-${GIT_VERSION} && \
    make configure && \
    ./configure --prefix=/usr/local --with-python=`which python3` && \
    PYTHON_PATH=`which python3` NO_TCLTK=1 make all && \
    make DESTDIR=/git install && \
    rm -rf /git-${GIT_VERSION} && \
    microdnf remove emacs-filesystem autoconf

FROM $BASE_IMAGE

COPY --from=0 / /
COPY --chmod=777 entrypoint /

ENV FIPS_ENABLED=1

ENV HOME /home/gitlab-runner

# https://docs.openshift.com/container-platform/4.6/openshift_images/create-images.html#support-arbitrary-user-ids
RUN mkdir -p /etc/gitlab-runner/certs $HOME /secrets && \
    chgrp -R 0 /etc/gitlab-runner && \
    chmod -R g=u /etc/gitlab-runner && \
    chgrp -R 0 $HOME && \
    chmod -R g=u $HOME && \
    chgrp -R 0 /secrets && \
    chmod -R g=u /secrets

USER 1001

STOPSIGNAL SIGQUIT
VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
CMD ["run", "--user=gitlab-runner", "--working-directory=/home/gitlab-runner"]
