package testcli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/tests/integration/mock_gitlab"
)

const TestConfig = "test-config.toml"

func TestMain(m *testing.M) {
	go mock_gitlab.Start()
	os.Exit(m.Run())
}

func TestConfigFileGeneration(t *testing.T) {
	const commonFlags = "--non-interactive --url http://localhost:8080 --config " + TestConfig + " --registration-token 1234567890 "
	tests := map[string]struct {
		flags string
	}{
		"basic invocation": {
			flags: commonFlags + "--name runna --executor docker --docker-image alpine:latest --docker-tlsverify=false --docker-privileged=false --docker-shm-size 0 --output-limit=40000 --docker-cpuset-cpus 1",
		},
		"basic invocation with short flags": {
			flags: "-n -u=http://localhost:8080 -c " + TestConfig + " --name short-flags -r 1234567890 --executor shell",
		},
		"ssh values": {
			flags: commonFlags + "--name runna --executor ssh --ssh-password supersecret --ssh-user root --ssh-host localhost --ssh-port 42 --ssh-identity-file id_rsa.pub",
		},
		"tls cert values": {
			flags: commonFlags + "--name tls --executor shell --tls-ca-file ca.file --tls-cert-file cert.file --tls-key-file key.file",
		},
		"docker advanced": {
			flags: commonFlags + "--name docker-advanced --executor docker --docker-image ubuntu:latest --docker-tlsverify --docker-privileged --docker-shm-size 1024 --output-limit=4321 --docker-cpuset-cpus 16 --docker-host localhost --docker-cert-path foo/bar --docker-hostname docker-hostname --docker-runtime test-runtime --docker-memory 2g --docker-memory-swap 512m --docker-memory-reservation 4096k --docker-cpus 8 --docker-cpu-shares 2 --docker-dns 8.8.8.8,8.8.4.4 --docker-dns-search gitlab.com --docker-disable-entrypoint-overwrite --docker-userns foonamespace --docker-cap-add NET_ADMIN --docker-cap-drop DAC_OVERRIDE --docker-oom-kill-disable --docker-oom-score-adjust 42 --docker-security-opt supersecure --docker-devices /dev/kvm --docker-disable-cache --docker-volumes /volumnes/foo --docker-volume-driver ext3 --docker-cache-dir ./foo/cache/dir/ --docker-extra-hosts registry.local:172.16.90.37 --docker-volumes-from /volumnes/inherited --docker-network-mode test-network --docker-links mysql_container:mysql --docker-wait-for-services-timeout 60 --docker-allowed-images ruby:*,python:*php:* --docker-allowed-services postgres:9,redis:*,mysql:* --docker-pull-policy if-not-present --docker-tmpfs /var/lib/mysql:rw,noexec --docker-services-tmpfs /var/lib/mysql:rw,noexec --docker-sysctls net.ipv4.ip_forward:1 --docker-helper-image helpme ",
		},
		"parallels advanced": {
			flags: commonFlags + "--name parallels-advanced --executor parallels --parallels-base-name windows-10-base --parallels-template-name windows-10-templ --parallels-disable-snapshots --parallels-time-server localhost.ntp --ssh-host localhost",
		},
		"virtualbox advanced": {
			flags: commonFlags + "--name virtualbox-advanced --executor virtualbox --virtualbox-base-name windows-10-base --virtualbox-base-snapshot windows-10-templ --virtualbox-disable-snapshots --ssh-host localhost --ssh-user test-user",
		},
		"cache s3": {
			flags: commonFlags + "--name cache --executor shell --cache-type s3 --cache-path our-cache --cache-shared --cache-s3-server-address s3-host:123 --cache-s3-access-key s3AccessKey --cache-s3-secret-key s3SecretKey --cache-s3-bucket-name cache-bucket --cache-s3-bucket-location us-east-1 --cache-s3-insecure",
		},
		"cache gcs": {
			flags: commonFlags + "--name cache --executor shell --cache-type gcs --cache-path our-cache --cache-gcs-access-id runner-id --cache-gcs-private-key private.key --cache-gcs-credentials-file credentials.file --cache-gcs-bucket-name gcs-cache-bucket",
		},
		"kubernetes": {
			flags: commonFlags + "--name k8s --executor kubernetes --kubernetes-host k8s.host --kubernetes-cert-file cert.file --kubernetes-key-file key.file --kubernetes-ca-file ca.file --kubernetes-bearer_token_overwrite_allowed --kubernetes-bearer_token t0ken --kubernetes-image alpine:latest --kubernetes-namespace k8s-ns --kubernetes-namespace_overwrite_allowed ci-${CI_COMMIT_REF_SLUG} --kubernetes-privileged --kubernetes-cpu-limit 1.0 --kubernetes-memory-limit 2g --kubernetes-service-cpu-limit 1 --kubernetes-service-memory-limit 512m --kubernetes-helper-cpu-limit 1 --kubernetes-helper-memory-limit 256m --kubernetes-cpu-request 1 --kubernetes-memory-request 256m --kubernetes-service-cpu-request 1 --kubernetes-service-memory-request 256m --kubernetes-helper-cpu-request 2 --kubernetes-helper-memory-request 1g --kubernetes-pull-policy always --kubernetes-node-selector foo.bar.com/node-pool:staging --kubernetes-node-tolerations key=value:effect --kubernetes-image-pull-secrets secret1,secret2 --kubernetes-helper-image helpMe:latest --kubernetes-terminationGracePeriodSeconds 42 --kubernetes-poll-interval 42 --kubernetes-poll-timeout 42 --kubernetes-pod-labels app:runner --kubernetes-service-account pod-acct --kubernetes-service_account_overwrite_allowed foo --kubernetes-pod-annotations anno:tation --kubernetes-pod_annotations_overwrite_allowed bar --kubernetes-pod-security-context-fs-group 42 --kubernetes-pod-security-context-run-as-group 99 --kubernetes-pod-security-context-run-as-non-root true --kubernetes-pod-security-context-run-as-user 1 --kubernetes-pod-security-context-supplemental-groups 5",
		},
		"docker machine": {
			flags: commonFlags + "--name machine --executor docker+machine --docker-image fuzz:latest --machine-idle-nodes 50 --machine-idle-time 60 --machine-max-builds 13 --machine-machine-driver amazon-ec2 --machine-machine-name runner-worker-%s --machine-machine-options digitalocean-image=coreos-stable",
		},
		"custom": {
			flags: commonFlags + "--name custom --executor custom --custom-config-exec /bin/config --custom-config-args foo,bar --custom-config-exec-timeout 300 --custom-prepare-exec /bin/prepare --custom-prepare-args baz,buz --custom-prepare-exec-timeout 600 --custom-run-exec /bin/run --custom-run-args aaa,bbb --custom-cleanup-exec /bin/tidyup --custom-cleanup-args cc,dd --custom-cleanup-exec-timeout 180 --custom-graceful-kill-timeout 30 --custom-force-kill-timeout 60",
		},
		"powershell": {
			flags: commonFlags + "--name powershell --registration-token 1234567890 --executor shell --builds-dir /my/builds --cache-dir /my/cache --clone-url foo.git --env ONE=TWO --pre-clone-script ./pre-clone.sh --pre-build-script ./pre-build.sh --post-build-script ./post-build.sh --debug-trace-disabled --shell powershell --custom_build_dir-enabled",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cleanUp()

			err := registerRunner(tt.flags)
			assert.NoError(t, err)
			assert.NoError(t, diffConfigs(expectedFileNameFromTestname(tn)))
			cleanUp()
		})
	}
}

func TestBools(t *testing.T) {
	cleanUp()
	mock_gitlab.ClearExpectations()
	mock_gitlab.SetExpectation("run_untagged", "true")
	mock_gitlab.SetExpectation("locked", "true")
	mock_gitlab.SetExpectation("active", "false")

	err := registerRunner("--non-interactive --url=http://localhost:8080 --config " + TestConfig + " --name bools-true --registration-token 1234567890 --executor shell --custom_build_dir-enabled --paused --locked --leave-runner --run-untagged")
	assert.NoError(t, err)
	assert.NoError(t, diffConfigs(expectedFileNameFromTestname(t.Name())))

	mock_gitlab.ClearExpectations()
	cleanUp()
}

func TestRunUntagged(t *testing.T) {
	cleanUp()
	mock_gitlab.ClearExpectations()
	mock_gitlab.SetExpectation("run_untagged", "true")

	const commonFlags = "--non-interactive --url=http://localhost:8080 --config test-config.toml --registration-token 1234567890 --executor shell "
	err := registerRunner(commonFlags + "--name run-untagged-1  --run-untagged --tag-list foo,bar")
	assert.NoError(t, err)
	err = registerRunner(commonFlags + "--name run-untagged-2 --run-untagged")
	assert.NoError(t, err)
	err = registerRunner(commonFlags + "--name run-untagged-3")
	assert.NoError(t, err)

	mock_gitlab.SetExpectation("run_untagged", "false")
	err = registerRunner(commonFlags + "--name run-untagged-4 --tag-list foo,bar")
	assert.NoError(t, err)

	mock_gitlab.ClearExpectations()
	cleanUp()
}
