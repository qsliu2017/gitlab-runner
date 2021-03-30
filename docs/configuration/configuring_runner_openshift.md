---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#designated-technical-writers
---

# Configuring GitLab Runner on OpenShift 

This document explains how to configure the GitLab Runner on OpenShift.

## Operator properties

This is a list of the supported properties that can be passed to the Operator.

| Setting | Description |
| ------- | ----------- |
| `gitlabUrl`     | The fully qualified domain name for the GitLab instance. E.g. `https://gitlab.example.com`. |
| `token`         | Name of `Secret` containing the `runner-registration-token` key used to register the runner. |
| `tags`          | List of comma separated tags to be applied to the runner. |
| `concurrent`    | Limits how many jobs can run concurrently. The maximum number is all defined runners. 0 does not mean unlimited. Default: 10 |
| `interval`      | Defines the number of seconds between checks for new jobs. Default: 30 |
| `cloneURL`      | Overwrite the URL for the GitLab instance. Used only if the runner can’t connect to the GitLab URL. |
| `env`           | Name of `ConfigMap` containing key-value pairs that will be injected as environment variables in the Runner pod. |
| `helperImage`   | Overwrites the default GitLab Runner Helper Image. |
| `buildImage`    | The default docker image to use for builds when none is specified. |
| `cacheType`     | Type of cache used for Runner artifacts. One of: `gcs`, `s3`, `azure`. |
| `cachePath`     | Defines the cache path on the file system. |
| `cacheShared`   | Enable sharing of cache between Runners. |
| `s3`            | Options used to setup S3 cache. Refer to [Cache properties](#cache-properties). |
| `gcs`           | Options used to setup GCS cache. Refer to [Cache properties](#cache-properties). |
| `azure`         | Options used to setup Azure cache. Refer to [Cache properties](#cache-properties). |
| `ca`            | Name of tls secret containing the custom certificate authority (CA) certificates. |
| `serviceAccount`| Allows to override service account used to run the Runner pod. |
| `config`        | Allows to provide a custom config map with a [configuration template](../register/index.md#runners-configuration-template-file). |

## Cache properties

### S3 cache

| Setting | Description |
| ------- | ----------- |
| `server`        | The S3 server address. |
| `credentials`   | Name of the `Secret` containing the `accesskey` and `secretkey` properties used to access the object storage. |
| `bucket`        | Name of the bucket in which the cache will be stored. |
| `location`      | Name of the S3 region which the cache will be stored. |
| `insecure`      | Use insecure connections or HTTP |

### GCS cache

| Setting | Description |
| ------- | ----------- |
| `credentials`     | Name of the `Secret` containing the `access-id` and `private-key` properties used to access the object storage. |
| `bucket`          | Name of the bucket in which the cache will be stored. |
| `credentialsFile` | Takes GCS credentials file, 'keys.json'. |

### Azure cache

| Setting | Description |
| ------- | ----------- |
| `credentials`     | Name of the `Secret` containing the `accountName` and `privateKey` properties used to access the object storage. |
| `container`       | Name of the Azure container in which the cache will be stored. |
| `storageDomain`   | The domain name of the Azure blob storage. |

## Configure a proxy environment

To create a proxy environment:

1. Edit the `custom-env.yaml` file. For example:
    
   ```yaml
   apiVersion: v1
   data:
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
   ```
    
1. Update OpenShift to apply the changes.
    
   ```shell
   oc apply -f custom-env.yaml
   ``` 

1. Update your [`gitlab-runner.yml`](../install/openshift.md#install-gitlab-runner) file.

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret # Name of the secret containing the Runner token
     env: custom-env
   ```

## Customize config.toml with a configuration template

Using the [configuration template](../register/index.md#runners-configuration-template-file) the `config.toml` of the Runner can be customized.

1. Create a custom config template file. For example, let's instruct our Runner to mount an `EmptyDir` volume. Create the `custom-config.toml` file:

   ```toml
   [[runners]]
     [runners.kubernetes]
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "empty-dir"
           mount_path = "/path/to/empty_dir"
           medium = "Memory"
   ```

1. Create a `ConfigMap` name `custom-config-toml` from our `custom-config.toml` file:

   ```shell
    oc create configmap custom-config-toml --from-file config.toml=custom-config.toml
   ```

1. Set the `config` property of the `Runner`:

    ```yaml
    apiVersion: apps.gitlab.com/v1beta2
    kind: Runner
    metadata:
      name: dev
    spec:
      gitlabUrl: https://gitlab.example.com
      token: gitlab-runner-secret
      config: custom-config-toml
    ``` 

## Configure a custom TLS cert

1. To set a custom TLS cert, create a secret with key `tls.crt`. The file could be named `custom-tls-ca-secret.yaml`.:

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
        name: custom-tls-ca
    type: Opaque
    stringData:
        tls.ca: |
            -----BEGIN CERTIFICATE-----
            MIIEczCCA1ugAwIBAgIBADANBgkqhkiG9w0BAQQFAD..AkGA1UEBhMCR0Ix
            .....
            7vQMfXdGsRrXNGRGnX+vWDZ3/zWI0joDtCkNnqEpVn..HoX
            -----END CERTIFICATE-----
    ```

1. Create the secret:
    
   ```shell
   oc apply -f custom-tls-ca-secret.yaml
   ```

1. Set the `ca` key in the `runner.yaml` to the same name as the name of our secret:

    ```yaml
    apiVersion: apps.gitlab.com/v1beta2
    kind: Runner
    metadata:
      name: dev
    spec:
      gitlabUrl: https://gitlab.example.com
      token: gitlab-runner-secret
      ca: custom-tls-ca
    ``` 

## Configure the cpu and memory size of runner pods

To set [CPU limits](../executors/kubernetes.md#cpu-requests-and-limits) and [memory limits](../executors/kubernetes.md#memory-requests-and-limits) in a custom `config.toml` file, follow the instructions in [this topic](#add-root-ca-certs-to-runners).

## Configure job concurrency per runner based on cluster resources

Set the `concurrent` property of the `Runner` resource: 

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     concurrent: 2
   ```

Job concurrency is dictated by the requirements of the specific project. 

1. Start by trying to determine the compute and memory resources required to execute a CI job.
1. Calculate how many times that job would be able to execute given the resources in the cluster.

If you set too large a concurrency value, the Kubernetes executor will process the jobs as soon as it can,
however when the jobs will be scheduled by the Kubernetes cluster's scheduler itself depends on its capacity.  
