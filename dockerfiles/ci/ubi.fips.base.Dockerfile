ARG UBI_VERSION

FROM redhat/ubi9-minimal:${UBI_VERSION}

ARG PLATFORM_ARCH=amd64

RUN INSTALL_PKGS="tar gzip perl" &&  \
    microdnf update -y && \
    microdnf install -y --setopt=tsflags=nodocs $INSTALL_PKGS && \
    microdnf clean all -y

ARG OPENSSL_VERSION

RUN curl -Lo openssl-${OPENSSL_VERSION}.tar.gz https://www.openssl.org/source/openssl-${OPENSSL_VERSION}.tar.gz && \
    tar -xf openssl-${OPENSSL_VERSION}.tar.gz && \
    cd openssl-${OPENSSL_VERSION} && \
    ./Configure enable-fips && \
    make install && \
    openssl version -v

COPY dockerfiles/ci/verify_load_fips_modules.c /verify_load_fips_modules.c
RUN  gcc -o /verify_load_fips_modules -L/usr/local/ssl/lib -lssl -lcrypto /verify_load_fips_modules.c && \
     /verify_load_fips_modules && \
     rm /verify_load_fips_modules /verify_load_fips_modules.c

