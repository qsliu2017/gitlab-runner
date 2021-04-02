package kubernetes

//
//import (
//	"encoding/json"
//	"errors"
//	"fmt"
//
//	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
//
//	"k8s.io/client-go/kubernetes"
//
//	dockerCliTypes "github.com/docker/cli/cli/config/types"
//
//	"gitlab.com/gitlab-org/gitlab-runner/common"
//
//	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
//	api "k8s.io/api/core/v1"
//)
//
//type buildPod struct {
//	pod         *api.Pod
//	configMap   *api.ConfigMap
//	credentials *api.Secret
//	services    []api.Service
//}
//
//type buildPodsProvider struct {
//	kubeClient *kubernetes.Clientset
//
//	build                   *common.Build
//	shell                   *common.ShellScriptInfo
//	configurationOverwrites *overwrites
//	options                 *kubernetesOptions
//	config                  *common.RunnerConfig
//	pullPolicy              common.KubernetesPullPolicy
//}
//
//func (p *buildPodsProvider) provision() (*buildPod, error) {
//	buildPod := &buildPod{}
//
//	err := p.setupCredentials(buildPod)
//	if err != nil {
//		return nil, err
//	}
//
//	err = p.setupBuildPod(buildPod, nil)
//	if err != nil {
//		return nil, err
//	}
//
//	panic("")
//}
//
//func (p *buildPodsProvider) setupCredentials(buildPod *buildPod) error {
//	//s.Debugln("Setting up secrets")
//
//	authConfigs, err := auth.ResolveConfigs(p.build.GetDockerAuthConfig(), p.shell.User, p.build.Credentials)
//	if err != nil {
//		return err
//	}
//
//	if len(authConfigs) == 0 {
//		return nil
//	}
//
//	dockerCfgs := make(map[string]dockerCliTypes.AuthConfig)
//	for registry, registryInfo := range authConfigs {
//		dockerCfgs[registry] = registryInfo.AuthConfig
//	}
//
//	dockerCfgContent, err := json.Marshal(dockerCfgs)
//	if err != nil {
//		return err
//	}
//
//	secret := api.Secret{}
//	secret.GenerateName = p.build.ProjectUniqueName()
//	secret.Namespace = p.configurationOverwrites.namespace
//	secret.Type = api.SecretTypeDockercfg
//	secret.Data = map[string][]byte{}
//	secret.Data[api.DockerConfigKey] = dockerCfgContent
//
//	creds, err := p.kubeClient.CoreV1().Secrets(p.configurationOverwrites.namespace).Create(&secret)
//	if err != nil {
//		return err
//	}
//
//	buildPod.credentials = creds
//	return nil
//}
//
//func (p *buildPodsProvider) setupBuildPod(buildPod *buildPod, initContainers []api.Container) error {
//	//s.Debugln("Setting up build pod")
//
//	podServices := make([]api.Container, len(p.options.Services))
//
//	for i, service := range p.options.Services {
//		resolvedImage := p.build.GetAllVariables().ExpandValue(service.Name)
//		podServices[i] = p.buildContainer(
//			fmt.Sprintf("svc-%d", i),
//			resolvedImage,
//			service,
//			p.configurationOverwrites.serviceRequests,
//			p.configurationOverwrites.serviceLimits,
//		)
//	}
//	// We set a default label to the pod. This label will be used later
//	// by the services, to link each service to the pod
//	labels := map[string]string{"pod": p.build.ProjectUniqueName()}
//	for k, v := range p.build.Runner.Kubernetes.PodLabels {
//		labels[k] = p.build.Variables.ExpandValue(v)
//	}
//
//	annotations := make(map[string]string)
//	for key, val := range p.configurationOverwrites.podAnnotations {
//		annotations[key] = p.build.Variables.ExpandValue(val)
//	}
//
//	var imagePullSecrets []api.LocalObjectReference
//	for _, imagePullSecret := range p.config.Kubernetes.ImagePullSecrets {
//		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
//	}
//
//	if buildPod.credentials != nil {
//		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: buildPod.credentials.Name})
//	}
//
//	hostAliases, err := p.getHostAliases()
//	if err != nil {
//		return err
//	}
//
//	podConfig := p.preparePodConfig(labels, annotations, podServices, imagePullSecrets, hostAliases, initContainers)
//
//	//s.Debugln("Creating build pod")
//	pod, err := p.kubeClient.CoreV1().Pods(p.configurationOverwrites.namespace).Create(&podConfig)
//	if err != nil {
//		return err
//	}
//
//	buildPod.pod = pod
//	buildPod.services, err = p.makePodProxyServices()
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (p *buildPodsProvider) getHostAliases() ([]api.HostAlias, error) {
//	supportsHostAliases, err := s.featureChecker.IsHostAliasSupported()
//	switch {
//	case errors.Is(err, &badVersionError{}):
//		s.Warningln("Checking for host alias support. Host aliases will be disabled.", err)
//		return nil, nil
//	case err != nil:
//		return nil, err
//	case !supportsHostAliases:
//		return nil, nil
//	}
//
//	return createHostAliases(s.options.Services, s.Config.Kubernetes.GetHostAliases())
//}
//
//func (p *buildPodsProvider) buildContainer(
//	name, image string,
//	imageDefinition common.Image,
//	requests, limits api.ResourceList,
//	containerCommand ...string,
//) api.Container {
//	privileged := false
//	var allowPrivilegeEscalation *bool
//	containerPorts := make([]api.ContainerPort, len(imageDefinition.Ports))
//	proxyPorts := make([]proxy.Port, len(imageDefinition.Ports))
//
//	for i, port := range imageDefinition.Ports {
//		proxyPorts[i] = proxy.Port{Name: port.Name, Number: port.Number, Protocol: port.Protocol}
//		containerPorts[i] = api.ContainerPort{ContainerPort: int32(port.Number)}
//	}
//
//	if len(proxyPorts) > 0 {
//		serviceName := imageDefinition.Alias
//
//		if serviceName == "" {
//			serviceName = name
//			if name != buildContainerName {
//				serviceName = fmt.Sprintf("proxy-%s", name)
//			}
//		}
//
//		s.ProxyPool[serviceName] = s.newProxy(serviceName, proxyPorts)
//	}
//
//	if p.config.Kubernetes != nil {
//		privileged = p.config.Kubernetes.Privileged
//		allowPrivilegeEscalation = p.config.Kubernetes.AllowPrivilegeEscalation
//	}
//
//	command, args := p.getCommandAndArgs(imageDefinition, containerCommand...)
//
//	return api.Container{
//		Name:            name,
//		Image:           image,
//		ImagePullPolicy: api.PullPolicy(p.pullPolicy),
//		Command:         command,
//		Args:            args,
//		Env:             buildVariables(p.build.GetAllVariables().PublicOrInternal()),
//		Resources: api.ResourceRequirements{
//			Limits:   limits,
//			Requests: requests,
//		},
//		Ports:        containerPorts,
//		VolumeMounts: p.getVolumeMounts(),
//		SecurityContext: &api.SecurityContext{
//			Privileged:               &privileged,
//			AllowPrivilegeEscalation: allowPrivilegeEscalation,
//			Capabilities: getCapabilities(
//				GetDefaultCapDrop(),
//				p.config.Kubernetes.CapAdd,
//				p.config.Kubernetes.CapDrop,
//			),
//		},
//		Stdin: true,
//	}
//}
