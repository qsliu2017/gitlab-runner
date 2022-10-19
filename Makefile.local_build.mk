.PHONY: check-dependencies
check-dependencies: GOX_HELP := $(shell gox --help > /dev/null 2>&1; echo $$?)
check-dependencies: DOCKER_BUILDX := $(shell docker buildx --help > /dev/null 2>&1; echo $$?)
check-dependencies: GEM_LIST := $(shell gem --help > /dev/null 2>&1; echo $$?)
check-dependencies: FPM_FOUND := $(shell fpm --help > /dev/null 2>&1; echo $$?)
check-dependencies: ZLIB_FOUND := $(shell gem list --local | grep zlib)
check-dependencies:
	@if [ "$(GOX_HELP)" != "0" ]; then echo "Executing make gox..."; make gox; echo "Add ~/go/bin in your env PATH..."; exit 1; fi
	@if [ "$(DOCKER_BUILDX)" != "0" ]; then echo "docker buildx is not installed"; exit 1; fi
	@if [ "$(GEM_LIST)" != "0" ]; then echo "Ruby seems to not be installed we need fpm and zlib to continue. If you are running on linux you might also need to run 'apt install build-essential dh-autoreconf'"; exit 1; fi
	@if [ "$(FPM_FOUND)" != "0" ]; then echo "You need to install FPM: 'gem install fpm' should do"; exit 1; fi
	@if [ -z "$ZLIB_FOUND" ]; then echo "You need to install ZLIB: 'gem install zlib' should do"; exit 1; fi
	@echo "All dependencies needed were found"

.PHONY: runner-and-helper-deb-host-arch
runner-and-helper-deb-host-arch: ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/386/)
runner-and-helper-deb-host-arch: export BUILD_ARCHS := -arch '$(ARCH)'
runner-and-helper-deb-host-arch: PACKAGE_ARCH := $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/i686/)
runner-and-helper-deb-host-arch: runner-and-helper-bin-host
	$(MAKE) package-prepare
	$(MAKE) package-deb-arch-subset ARCH=$(ARCH) PACKAGE_ARCH=$(PACKAGE_ARCH)

.PHONY: ra-test
ra-test:
#	@./ci/package_temp $(shell uname -m | sed s/x86_64/amd64/ | sed s/i386/386/)
#	$(ARCH) $(PACKAGE_ARCH)

.PHONY: package-deb-arch-subset
package-deb-arch-subset: ARCH ?= amd64
package-deb-arch-subset: export PACKAGE_ARCH ?= amd64
package-deb-arch-subset: export RUNNER_BINARY ?= out/binaries/$(NAME)-linux-$(ARCH)
package-deb-arch-subset:
	@./ci/local/package $(ARCH)

release-docker-images-subset:
	# Releasing GitLab Runner images
	@./ci/local/release_docker_images

release-helper-docker-images-subset:
	# Releasing GitLab Runner Helper images
	@./ci/local/release_helper_docker_images

runner-and-helper-docker-host-arch: check-dependencies
runner-and-helper-docker-host-arch: export PUBLISH_IMAGES=true
runner-and-helper-docker-host-arch: export PUSH_TO_ECR_PUBLIC=false
runner-and-helper-docker-host-arch: export PUSH_TO_DOCKER_HUB=true
runner-and-helper-docker-host-arch: export DOCKER_HUB_NAMESPACE=gitlab
runner-and-helper-docker-host-arch: export CI_REGISTRY_IMAGE=gitlab
runner-and-helper-docker-host-arch: export ALPINE_312_VERSION=3.12.12
runner-and-helper-docker-host-arch: export ALPINE_313_VERSION=3.13.12
runner-and-helper-docker-host-arch: export ALPINE_314_VERSION=3.14.8
runner-and-helper-docker-host-arch: export ALPINE_315_VERSION=3.15.6
runner-and-helper-docker-host-arch: export ALPINE_LATEST_VERSION=latest
runner-and-helper-docker-host-arch: export UBUNTU_VERSION=20.04
runner-and-helper-docker-host-arch: export CI_COMMIT_REF_SLUG=$(shell echo $(BRANCH) | cut -c -63 | sed -E 's/[^a-z0-9-]+/-/g' | sed -E 's/^-*([a-z0-9-]+[a-z0-9])-*$$/\1/g')
runner-and-helper-docker-host-arch:
	$(MAKE) runner-and-helper-deb-host-arch
	$(MAKE) release-docker-images-subset
#	$(MAKE) release-helper-docker-images-subset
