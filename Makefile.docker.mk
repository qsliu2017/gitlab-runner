docker: helper-images/prebuilt-x86_64.tar.xz helper-images/prebuilt-arm.tar.xz

HELPER_GO_FILES ?= $(shell find core apps/gitlab-runner-helper -name '*.go')

GO_x86_64_ARCH = amd64
GO_arm_ARCH = arm

helper-images:
	mkdir -p $@

helper-images/prebuilt-%.tar.xz: helper-images/prebuilt-%.tar
ifneq (, $(shell docker info))
	xz -f -9 $<
else
	$(warning WARNING: downloading prebuilt docker images that will be loaded by gitlab-runner)
	curl -o $@ https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/$(shell basename $@)
endif


dockerfiles/build/gitlab-runner-helper.%: $(HELPER_GO_FILES) $(GOX)
	gox -osarch=linux/$(GO_$*_ARCH) -ldflags "$(GO_LDFLAGS)" -output=$@ $(PKG)/apps/gitlab-runner-helper

helper-images/prebuilt-%.tar: helper-images dockerfiles/build/gitlab-runner-helper.%
ifneq (, $(shell docker info))
	docker build -t gitlab/gitlab-runner-helper:$*-$(REVISION) -f dockerfiles/build/Dockerfile.$* dockerfiles/build
	-docker rm -f gitlab-runner-prebuilt-$*-$(REVISION)
	docker create --name=gitlab-runner-prebuilt-$*-$(REVISION) gitlab/gitlab-runner-helper:$*-$(REVISION) /bin/sh
	docker export -o $@ gitlab-runner-prebuilt-$*-$(REVISION)
	docker rm -f gitlab-runner-prebuilt-$*-$(REVISION)
else
	$(warning WARNING: Docker Engine is missing, cannot compile helper images)
endif

