ARG UBI_IMAGE=registry.access.redhat.com/ubi8/ubi:8.1

FROM ${UBI_IMAGE}

ADD https://github.com/Yelp/dumb-init/releases/download/v1.0.2/dumb-init_1.0.2_amd64 /usr/bin/dumb-init

RUN echo $'[runner_gitlab-runner] \n\
name=runner_gitlab-runner \n\
baseurl=https://packages.gitlab.com/runner/gitlab-runner/el/7/$basearch \n\
repo_gpgcheck=0 \n\
gpgcheck=0 \n\
enabled=1 \n\
gpgkey=https://packages.gitlab.com/gpg.key \n\
sslverify=1 \n\
sslcacert=/etc/pki/tls/certs/ca-bundle.crt \n\
metadata_expire=300 \n\
[runner_gitlab-runner-source] \n\
name=runner_gitlab-runner-source \n\
baseurl=https://packages.gitlab.com/runner/gitlab-runner/el/7/SRPMS \n\
repo_gpgcheck=0 \n\
gpgcheck=0 \n\
enabled=1 \n\
gpgkey=https://packages.gitlab.com/gpg.key \n\
sslverify=1 \n\
sslcacert=/etc/pki/tls/certs/ca-bundle.crt \n\
metadata_expire=300' > /etc/yum.repos.d/runner_gitlab-runner.repo 

RUN dnf --disableplugin=subscription-manager makecache -y && \
    dnf --disableplugin=subscription-manager install -y gitlab-runner && \
    rm -rf /var/lib/apt/lists/*


RUN mkdir -p /etc/gitlab-runner/certs && \
    mkdir /.gitlab-runner && \
    mkdir /tmp/gitlab-home && \
    chmod -R g=u /.gitlab-runner && \
    chmod -R g=u /etc/gitlab-runner && \
    chmod -R g=u /tmp/gitlab-home && \
    chmod +x /usr/bin/dumb-init

ADD entrypoint /
RUN chmod +x /entrypoint

VOLUME ["/etc/gitlab-runner", "/home/gitlab-runner"]
ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]
