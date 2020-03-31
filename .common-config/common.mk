export REVISION ?= $(shell git rev-parse --short HEAD || echo "unknown")
export BRANCH ?= $(shell git show-ref | grep "$(REVISION)" | grep -v HEAD | awk '{print $$2}' | sed 's|refs/remotes/origin/||' | sed 's|refs/heads/||' | sort | head -n 1)
export BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%S%z)

_common_mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
_common_config_dir := $(notdir $(patsubst %/,%,$(dir $(_common_mkfile_path))))

mockery ?= $(GOPATH)/bin/mockery
gox ?= $(GOPATH)/bin/gox
export goJunitReport ?= $(GOPATH)/bin/go-junit-report

lint: export TOOL_VERSION ?= 1.24.0
lint: export OUT_FORMAT ?= colored-line-number
lint: export LINT_FLAGS ?=
lint:
	@$(_common_config_dir)/scripts/lint

check_modules:
	# Check if there is any difference in vendor/
	@if [ -d vendor ]; then \
		git status -sb vendor/ > /tmp/vendor-$${CI_JOB_ID}-before; \
		go mod vendor; \
		git status -sb vendor/ > /tmp/vendor-$${CI_JOB_ID}-after; \
		diff -U0 /tmp/vendor-$${CI_JOB_ID}-before /tmp/vendor-$${CI_JOB_ID}-after; \
	fi

	# check go.sum
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-before
	@go mod tidy
	@git diff go.sum > /tmp/gosum-$${CI_JOB_ID}-after
	@diff -U0 /tmp/gosum-$${CI_JOB_ID}-before /tmp/gosum-$${CI_JOB_ID}-after

check_mocks:
	# Checking if mocks are up-to-date
	@$(MAKE) mocks
	# Checking the differences
	@git --no-pager diff --compact-summary --exit-code -- "$(MOCK_TARGETS)" && \
		echo "Mocks up-to-date!"

prepare_index: export CI_COMMIT_REF_NAME ?= $(BRANCH)
prepare_index: export CI_COMMIT_SHA ?= $(REVISION)
prepare_index: $(releaseIndexGen)
	# Preparing index file
	@$(releaseIndexGen) -working-directory "$(WORKING_DIRECTORY)" \
					   	-project-version "$(VERSION)" \
					   	-project-git-ref "$(CI_COMMIT_REF_NAME)" \
					   	-project-git-revision "$(CI_COMMIT_SHA)" \
					   	-project-name "$(PROJECT_NAME)" \
					   	-project-repo-url "$(PROJECT_REPO_URL)" \
					   	-gpg-key-env GPG_KEY \
					   	-gpg-password-env GPG_PASSPHRASE

$(mockery):
	# Installing mockery
	@go get github.com/vektra/mockery/cmd/mockery

$(gox):
	# Installing gox
	@go get github.com/mitchellh/gox

$(goJunitReport):
	# Installing go-junit-report
	@go get github.com/jstemmer/go-junit-report

$(releaseIndexGen): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
$(releaseIndexGen): DOWNLOAD_URL = "https://storage.googleapis.com/gitlab-runner-tools/release-index-generator/$(RELEASE_INDEX_GEN_VERSION)/release-index-gen-$(OS_TYPE)-amd64"
$(releaseIndexGen):
	# Installing $(DOWNLOAD_URL) as $(releaseIndexGen)
	@mkdir -p $(shell dirname $(releaseIndexGen))
	@curl -sL "$(DOWNLOAD_URL)" -o "$(releaseIndexGen)"
	@chmod +x "$(releaseIndexGen)"

$(gitlabChangelog): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
$(gitlabChangelog): DOWNLOAD_URL = "https://storage.googleapis.com/gitlab-runner-tools/gitlab-changelog/$(GITLAB_CHANGELOG_VERSION)/gitlab-changelog-$(OS_TYPE)-amd64"
$(gitlabChangelog):
	# Installing $(DOWNLOAD_URL) as $(gitlabChangelog)
	@mkdir -p $(shell dirname $(gitlabChangelog))
	@curl -sL "$(DOWNLOAD_URL)" -o "$(gitlabChangelog)"
	@chmod +x "$(gitlabChangelog)"

$(gitlabCommonConfig): OS_TYPE ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
$(gitlabCommonConfig): DOWNLOAD_URL = "https://storage.googleapis.com/gitlab-runner-tools/runner-common-config/$(GITLAB_COMMON_CONFIG_VERSION)/runner-common-config-$(OS_TYPE)-amd64"
$(gitlabCommonConfig):
	# Installing $(DOWNLOAD_URL) as $(gitlabCommonConfig)
	@mkdir -p $(shell dirname $(gitlabCommonConfig))
	@curl -sL "$(DOWNLOAD_URL)" -o "$(gitlabCommonConfig)"
	@chmod +x "$(gitlabCommonConfig)"
