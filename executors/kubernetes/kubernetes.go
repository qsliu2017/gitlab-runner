package kubernetes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/docker/cli/cli/config/types"
	"github.com/jpillora/backoff"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Register all available authentication methods
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/exec"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/samber/lo"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const (
	buildContainerName  = "build"
	helperContainerName = "helper"

	detectShellScriptName         = "detect_shell_script"
	pwshJSONTerminationScriptName = "terminate_with_json_script"

	waitLogFileTimeout = time.Minute

	outputLogFileNotExistsExitCode = 100
	unknownLogProcessorExitCode    = 1000

	// nodeSelectorWindowsBuildLabel is the label used to reference a specific Windows Version.
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesiowindows-build
	nodeSelectorWindowsBuildLabel = "node.kubernetes.io/windows-build"

	apiVersion         = "v1"
	ownerReferenceKind = "Pod"

	// Polling time between each attempt to check serviceAccount and imagePullSecret (in seconds)
	resourceAvailabilityCheckMaxPollInterval = 5 * time.Second

	serviceContainerPrefix = "svc-"

	k8sAnnotationPrefix = "runner.gitlab.com/"

	defaultTries           = 5
	defaultRetryMinBackoff = 500 * time.Millisecond
	defaultRetryMaxBackoff = 2 * time.Second

	// The suffix is built using alphanumeric character
	// that means there is 34^8 possibilities for a resource name using the same pattern
	// Considering that the k8s resources are deleted after they run,
	k8sResourcesNameSuffixLength = 8
	k8sResourcesNameMaxLength    = 63

	k8sEventWarningType = "Warning"

	// errorDialingBackendMessage is an error prefix that is encountered when
	// connectivity to a Pod fails. This can happen for a number of reasons,
	// such as the Pod or Node still being configured.
	errorDialingBackendMessage            = "error dialing backend"
	errorTLSHandshakeTimeoutMessage       = "TLS handshake timeout"
	errorUnexpectedEOFMessage             = "unexpected EOF"
	errorReadConnectionTimedOutMessage    = "read: connection timed out"
	errorConnectConnectionTimedOutMessage = "connect: connection timed out"
	errorTimeoutOccurredMessage           = "Timeout occurred"
	errorHTTP2ConnectionLostMessage       = "http2: client connection lost"

	// errorAlreadyExistsMessage is an error message that is encountered when
	// we fail to create a resource because it already exists.
	// Because of a connectivity issue, an attempt to create a resource can fail while the request itself
	// was successfully executed. We then monitor the conflict error message to retrieve the already create resource
	errorAlreadyExistsMessage = "the server was not able to generate a unique name for the object"
)

var (
	PropagationPolicy = metav1.DeletePropagationBackground

	executorOptions = executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultSafeDirectoryCheckout:  true,
		DefaultBuildsDir:              "/builds",
		DefaultCacheDir:               "/cache",
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
	}

	errIncorrectShellType = fmt.Errorf("kubernetes executor incorrect shell type")

	DefaultResourceIdentifier  = "default"
	resourceTypeServiceAccount = "ServiceAccount"
	resourceTypePullSecret     = "ImagePullSecret"

	chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	errorRegex = fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s",
		errorDialingBackendMessage,
		errorTLSHandshakeTimeoutMessage,
		errorUnexpectedEOFMessage,
		errorReadConnectionTimedOutMessage,
		errorConnectConnectionTimedOutMessage,
		errorTimeoutOccurredMessage,
		errorHTTP2ConnectionLostMessage,
	)
)

type commandTerminatedError struct {
	exitCode int
}

func (c *commandTerminatedError) Error() string {
	return fmt.Sprintf("command terminated with exit code %d", c.exitCode)
}

func (c *commandTerminatedError) Is(err error) bool {
	_, ok := err.(*commandTerminatedError)
	return ok
}

type retryableKubeAPICall[T any] struct {
	*retry.Retry[T]
}

func newRetryableKubeAPICallWithValue[T any](fn retry.RunValueFunc[T]) *retryableKubeAPICall[T] {
	return &retryableKubeAPICall[T]{
		Retry: retry.
			NewWithValue(fn).
			WithCheck(func(_ int, err error) bool {
				return isNetworkError(err)
			}).
			WithMaxTries(defaultTries).
			WithBackoff(defaultRetryMinBackoff, defaultRetryMaxBackoff),
	}
}

func newRetryableKubeAPICall(fn retry.RunFunc) *retryableKubeAPICall[any] {
	return newRetryableKubeAPICallWithValue(fn.ToValueFunc())
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	matchError, _ := regexp.MatchString(errorRegex, err.Error())
	return errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		matchError
}

type podPhaseError struct {
	name  string
	phase api.PodPhase
}

func (p *podPhaseError) Error() string {
	return fmt.Sprintf("pod %q status is %q", p.name, p.phase)
}

type resourceCheckError struct {
	resourceType string
	resourceName string
}

func (r *resourceCheckError) Error() string {
	return fmt.Sprintf(
		"Timed out while waiting for %s/%s to be present in the cluster",
		r.resourceType,
		r.resourceName,
	)
}

func (r *resourceCheckError) Is(err error) bool {
	_, ok := err.(*resourceCheckError)
	return ok
}

type kubernetesOptions struct {
	Image    common.Image
	Services common.Services
}

type containerBuildOpts struct {
	name               string
	image              string
	imageDefinition    common.Image
	isServiceContainer bool
	requests           api.ResourceList
	limits             api.ResourceList
	securityContext    *api.SecurityContext
	command            []string
}

type podConfigPrepareOpts struct {
	labels           map[string]string
	annotations      map[string]string
	services         []api.Container
	initContainers   []api.Container
	imagePullSecrets []api.LocalObjectReference
	hostAliases      []api.HostAlias
}

type executor struct {
	executors.AbstractExecutor

	kubeClient  *kubernetes.Clientset
	kubeConfig  *restclient.Config
	pod         *api.Pod
	credentials *api.Secret
	options     *kubernetesOptions
	services    []api.Service

	configurationOverwrites *overwrites
	pullManager             pull.Manager

	helperImageInfo helperimage.Info

	featureChecker featureChecker

	newLogProcessor func() logProcessor

	remoteProcessTerminated chan shells.StageCommandStatus

	requireSharedBuildsDir *bool

	// Flag if a repo mount and emptyDir volume are needed
	requireDefaultBuildsDirVolume *bool

	remoteStageStatusMutex sync.Mutex
	remoteStageStatus      shells.StageCommandStatus

	eventsStream watch.Interface
}

type serviceCreateResponse struct {
	service *api.Service
	err     error
}

// nolint:funlen
func (s *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	s.AbstractExecutor.PrepareConfiguration(options)

	if err = s.prepareOverwrites(options.Build.GetAllVariables()); err != nil {
		return fmt.Errorf("couldn't prepare overwrites: %w", err)
	}

	s.pullManager, err = s.preparePullManager(options)
	if err != nil {
		return err
	}

	s.prepareOptions(options.Build)

	// Dynamically configure use of shared build dir allowing
	// for static build dir when isolated volume is in use.
	s.SharedBuildsDir = s.isSharedBuildsDirRequired()

	if err = s.checkDefaults(); err != nil {
		return fmt.Errorf("check defaults error: %w", err)
	}

	s.kubeConfig, err = getKubeClientConfig(s.Config.Kubernetes, s.configurationOverwrites)
	if err != nil {
		return fmt.Errorf("getting Kubernetes config: %w", err)
	}

	s.kubeClient, err = kubernetes.NewForConfig(s.kubeConfig)
	if err != nil {
		return fmt.Errorf("connecting to Kubernetes: %w", err)
	}

	s.helperImageInfo, err = s.prepareHelperImage()
	if err != nil {
		return fmt.Errorf("prepare helper image: %w", err)
	}

	// setup default executor options based on OS type
	s.setupDefaultExecutorOptions(s.helperImageInfo.OSType)

	s.featureChecker = &kubeClientFeatureChecker{kubeClient: s.kubeClient}

	imageName := s.options.Image.Name

	s.Println("Using Kubernetes executor with image", imageName, "...")
	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		s.Println("Using attach strategy to execute scripts...")
	}

	s.Debugln(fmt.Sprintf("Using helper image: %s:%s", s.helperImageInfo.Name, s.helperImageInfo.Tag))

	if err = s.AbstractExecutor.PrepareBuildAndShell(); err != nil {
		return fmt.Errorf("prepare build and shell: %w", err)
	}

	if s.BuildShell.PassFile {
		return fmt.Errorf("kubernetes doesn't support shells that require script file")
	}

	return err
}

func (s *executor) preparePullManager(options common.ExecutorPrepareOptions) (pull.Manager, error) {
	var (
		err                 error
		pullPolicies        []api.PullPolicy
		allowedPullPolicies []api.PullPolicy
	)

	pullPolicies, err = s.getPullPolicies(options.Build.Image.PullPolicies)
	if err != nil {
		return nil, err
	}

	allowedPullPolicies, err = s.Config.Kubernetes.GetAllowedPullPolicies()
	if err != nil {
		return nil, err
	}

	err = s.verifyPullPolicies(pullPolicies, allowedPullPolicies, options.Build.Image.PullPolicies)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image '%s': %w", options.Build.Image.Name, err)
	}

	return pull.NewPullManager(pullPolicies, &s.BuildLogger), nil
}

// getPullPolicies selects the pull_policy configurations originating from
// either gitlab-ci.yaml or config.toml. If present, the pull_policies in
// gitlab-ci.yaml take precedence over those in config.toml.
func (s *executor) getPullPolicies(imagePullPolicies []common.DockerPullPolicy) ([]api.PullPolicy, error) {
	k8sImagePullPolicies, err := s.Config.Kubernetes.ConvertFromDockerPullPolicy(imagePullPolicies)
	if err != nil {
		return nil, fmt.Errorf("conversion to Kubernetes policy: %w", err)
	}

	if len(k8sImagePullPolicies) != 0 {
		return k8sImagePullPolicies, nil
	}

	return s.Config.Kubernetes.GetPullPolicies()
}

func (s *executor) verifyPullPolicies(
	pullPolicies,
	allowedPullPolicies []api.PullPolicy,
	imagePullPolicies []common.DockerPullPolicy,
) error {
	for _, policy := range pullPolicies {
		for _, allowedPolicy := range allowedPullPolicies {
			if policy == allowedPolicy {
				return nil
			}
		}
	}

	return common.IncompatiblePullPolicyError(
		pullPolicies,
		allowedPullPolicies,
		common.GetPullPolicySource(imagePullPolicies, s.Config.Kubernetes.PullPolicy),
	)
}

func (s *executor) setupDefaultExecutorOptions(os string) {
	if os == helperimage.OSTypeWindows {
		s.DefaultBuildsDir = `C:\builds`
		s.DefaultCacheDir = `C:\cache`

		s.ExecutorOptions.Shell.Shell = shells.SNPowershell
		s.ExecutorOptions.Shell.RunnerCommand = "gitlab-runner-helper"
	}
}

func (s *executor) prepareHelperImage() (helperimage.Info, error) {
	config := s.retrieveHelperImageConfig()

	// use node selector labels to better select the correct image
	if s.Config.Kubernetes.NodeSelector != nil {
		for label, option := range map[string]*string{
			api.LabelArchStable:           &config.Architecture,
			api.LabelOSStable:             &config.OSType,
			nodeSelectorWindowsBuildLabel: &config.KernelVersion,
		} {
			value := s.Config.Kubernetes.NodeSelector[label]
			if value != "" {
				*option = value
			}
		}
	}

	// Also consider node selector overwrites as they may change arch or os
	if s.configurationOverwrites.nodeSelector != nil {
		for label, option := range map[string]*string{
			api.LabelArchStable:           &config.Architecture,
			api.LabelOSStable:             &config.OSType,
			nodeSelectorWindowsBuildLabel: &config.KernelVersion,
		} {
			value := s.configurationOverwrites.nodeSelector[label]
			if value != "" {
				*option = value
			}
		}
	}

	return helperimage.Get(common.REVISION, config)
}

func (s *executor) retrieveHelperImageConfig() helperimage.Config {
	cfg := helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
		Shell:        s.Config.Shell,
		Flavor:       s.ExpandValue(s.Config.Kubernetes.HelperImageFlavor),
	}

	if !s.Config.Kubernetes.HelperImageAutosetArchAndOS {
		return cfg
	}

	cfg.Architecture = common.AppVersion.Architecture
	if helperimage.OSTypeWindows == common.AppVersion.OS {
		cfg.OSType = helperimage.OSTypeWindows
	}

	return cfg
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	for attempt := 1; ; attempt++ {
		var err error

		if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
			s.Debugln("Starting Kubernetes command...")
			err = s.runWithExecLegacy(cmd)
		} else {
			s.Debugln("Starting Kubernetes command with attach...")
			err = s.runWithAttach(cmd)
		}

		if err != nil && s.Build.IsFeatureFlagOn(featureflags.RetrievePodWarningEvents) {
			s.logPodWarningEvents(k8sEventWarningType)
		}

		var imagePullErr *pull.ImagePullError
		if errors.As(err, &imagePullErr) {
			if s.pullManager.UpdatePolicyForImage(attempt, imagePullErr) {
				s.cleanupResources()
				s.pod = nil
				continue
			}
		}
		return err
	}
}

func (s *executor) handlePodEvents() error {
	// This will run the watcher only for the first call
	// which is when the pod is being initialized
	if s.eventsStream != nil {
		return nil
	}

	if err := s.watchPodEvents(); err != nil {
		return err
	}

	go s.printPodEvents()
	return nil
}

func (s *executor) watchPodEvents() error {
	s.BuildLogger.Println("Subscribing to Kubernetes Pod events...")
	// Continue polling for the status of the pod as that feels more straightforward than
	// checking for each individual container's status in the events.
	// It also makes it less likely to break something existing since we get the status of the Pod
	// when it's already failed.
	// This strategy can be revised in the future if needed.
	kubeRequest := newRetryableKubeAPICallWithValue(func() (watch.Interface, error) {
		// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
		return s.kubeClient.CoreV1().Events(s.pod.Namespace).Watch(context.Background(), metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", s.pod.Name),
		})
	})

	var err error
	s.eventsStream, err = kubeRequest.RunValue()
	return err
}

func (s *executor) printPodEvents() {
	w := tabwriter.NewWriter(s.Trace, 3, 1, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "Type\tReason\tMessage")

	// The s.eventsStream.Stop method will be called by the caller
	// that's how we'll exit from this loop
	for result := range s.eventsStream.ResultChan() {
		ev, ok := result.Object.(*api.Event)
		if !ok {
			continue
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", ev.Type, ev.Reason, ev.Message)
		_ = w.Flush()
	}
}

func (s *executor) logPodWarningEvents(eventType string) {
	if s.pod == nil {
		return
	}

	kubeRequest := newRetryableKubeAPICallWithValue(func() (*api.EventList, error) {
		// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
		return s.kubeClient.CoreV1().Events(s.pod.Namespace).
			List(context.Background(), metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s,type=%s", s.pod.Name, eventType),
			})
	})

	events, err := kubeRequest.RunValue()
	if err != nil {
		s.Errorln(fmt.Sprintf("Error retrieving events list: %s", err.Error()))
		return
	}

	for _, event := range events.Items {
		s.Warningln(fmt.Sprintf("Event retrieved from the cluster: %s", event.Message))
	}
}

func (s *executor) runWithExecLegacy(cmd common.ExecutorCommand) error {
	ctx := cmd.Context

	if err := s.setupPodLegacy(ctx); err != nil {
		return err
	}

	containerName := buildContainerName
	containerCommand := s.BuildShell.DockerCommand
	if cmd.Predefined {
		containerName = helperContainerName
		containerCommand = s.helperImageInfo.Cmd
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.Debugln(fmt.Sprintf(
		"Starting in container %q the command %q with script: %s",
		containerName,
		containerCommand,
		cmd.Script,
	))

	select {
	case err := <-s.runInContainerWithExec(ctx, containerName, containerCommand, cmd.Script):
		s.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: exitError.ExitStatus()}
		}
		return err

	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) setupPodLegacy(ctx context.Context) error {
	if s.pod != nil {
		return nil
	}

	err := s.setupCredentials(ctx)
	if err != nil {
		return err
	}

	err = s.setupBuildPod(ctx, nil)
	if err != nil {
		return err
	}

	if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		if err := s.handlePodEvents(); err != nil {
			return err
		}
	}

	return nil
}

func (s *executor) runWithAttach(cmd common.ExecutorCommand) error {
	ctx := cmd.Context

	err := s.ensurePodsConfigured(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	containerName, containerCommand := s.getContainerInfo(cmd)

	err = s.saveScriptOnEmptyDir(ctx, s.scriptName(string(cmd.Stage)), containerName, cmd.Script)
	if err != nil {
		return err
	}

	s.Debugln(fmt.Sprintf(
		"Starting in container %q the command %q with script: %s",
		containerName,
		containerCommand,
		cmd.Script,
	))

	podStatusCh := s.watchPodStatus(ctx)

	select {
	case err := <-s.runInContainer(ctx, cmd.Stage, containerName, containerCommand):
		s.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		var terminatedError *commandTerminatedError
		if err != nil && errors.As(err, &terminatedError) {
			return &common.BuildError{Inner: err, ExitCode: terminatedError.exitCode}
		}

		return err
	case err := <-podStatusCh:
		if IsKubernetesPodNotFoundError(err) {
			return err
		}

		return &common.BuildError{Inner: err}
	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) ensurePodsConfigured(ctx context.Context) error {
	if s.pod != nil {
		return nil
	}

	err := s.setupCredentials(ctx)
	if err != nil {
		return fmt.Errorf("setting up credentials: %w", err)
	}

	permissionsInitContainer, err := s.buildPermissionsInitContainer(s.helperImageInfo.OSType)
	if err != nil {
		return fmt.Errorf("building permissions init container: %w", err)
	}
	err = s.setupBuildPod(ctx, []api.Container{permissionsInitContainer})
	if err != nil {
		return fmt.Errorf("setting up build pod: %w", err)
	}

	if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		if err := s.handlePodEvents(); err != nil {
			return err
		}
	}

	var out io.Writer = s.Trace
	if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		out = io.Discard
	}

	status, err := waitForPodRunning(ctx, s.kubeClient, s.pod, out, s.Config.Kubernetes)
	if err != nil {
		return fmt.Errorf("waiting for pod running: %w", err)
	}

	if status != api.PodRunning {
		return fmt.Errorf("pod failed to enter running state: %s", status)
	}

	err = s.setupTrappingScripts(ctx)
	if err != nil {
		return fmt.Errorf("setting up trapping scripts on emptyDir: %w", err)
	}

	go s.processLogs(ctx)

	s.captureContainersLogs(ctx, s.pod.Spec.Containers)

	return nil
}

func (s *executor) getContainerInfo(cmd common.ExecutorCommand) (string, []string) {
	var containerCommand []string

	containerName := buildContainerName
	if cmd.Predefined {
		containerName = helperContainerName
	}

	shell := s.Shell().Shell

	switch shell {
	case shells.SNPwsh, shells.SNPowershell:
		// Translates to roughly "/path/to/parse_pwsh_script.ps1 /path/to/stage_script"
		containerCommand = []string{
			s.scriptPath(pwshJSONTerminationScriptName),
			s.scriptPath(cmd.Stage),
			s.buildRedirectionCmd(shell),
		}
	default:
		// Translates to roughly "sh -c '(/detect/shell/path.sh /stage/script/path.sh 2>&1 | tee) &'"
		// which when the detect shell exits becomes something like "bash /stage/script/path.sh".
		// This works unlike "gitlab-runner-build" since the detect shell passes arguments with "$@"
		containerCommand = []string{
			"sh",

			// We have to run the command in a background subshell. Unfortunately,
			// explaining why in a comment fails the code quality check of
			// function length not exceeding 60 lines, so `git blame` this instead.
			"-c",
			fmt.Sprintf("'(%s %s %s) &'",
				s.scriptPath(detectShellScriptName),
				s.scriptPath(cmd.Stage),
				s.buildRedirectionCmd(shell),
			),
		}
		if cmd.Predefined {
			// We use redirection here since the "gitlab-runner-build" helper doesn't pass input args
			// to the shell it executes, so we technically pass the script to the stdin of the underlying shell
			// translates roughly to "gitlab-runner-build <<< /stage/script/path.sh"
			containerCommand = append(
				s.helperImageInfo.Cmd,
				"<<<",
				s.scriptPath(cmd.Stage),
				s.buildRedirectionCmd(shell),
			)
		}
	}

	return containerName, containerCommand
}

func (s *executor) initContainerResources() api.ResourceRequirements {
	resources := api.ResourceRequirements{}

	if s.configurationOverwrites != nil {
		resources.Limits = s.configurationOverwrites.helperLimits
		resources.Requests = s.configurationOverwrites.helperRequests
	}

	return resources
}

func (s *executor) buildPermissionsInitContainer(os string) (api.Container, error) {
	pullPolicy, err := s.pullManager.GetPullPolicyFor(s.getHelperImage())
	if err != nil {
		return api.Container{}, fmt.Errorf("getting pull policy for permissions init container: %w", err)
	}

	container := api.Container{
		Name:            "init-permissions",
		Image:           s.getHelperImage(),
		VolumeMounts:    s.getVolumeMounts(),
		ImagePullPolicy: pullPolicy,
		// let's use build container resources
		Resources: s.initContainerResources(),
		SecurityContext: s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.InitPermissionsContainerSecurityContext,
			s.defaultCapDrop()...,
		),
	}

	// The kubernetes executor uses both a helper container (for predefined stages) and a build
	// container (for user defined steps). When accessing files on a shared volume, permissions
	// are resolved within the context of the individual container.
	//
	// For Linux, the helper container and build container can occasionally have the same user IDs
	// and access is not a problem. This can occur when:
	// - the image defines a user ID that is identical across both images
	// - PodSecurityContext is used and the UIDs is set manually
	// - Openshift is used and each pod is assigned a different user ID
	// Due to UIDs being different in other scenarios, we explicitly open the permissions on the
	// log shared volume so both containers have access.
	//
	// For Windows, the security identifiers are larger. Unlike Linux, its not likely to have
	// containers share the same identifier. The Windows Security Access Manager is not shared
	// between containers, so we need to open up permissions across more than just the logging
	// shared volume. Fortunately, Windows allows us to set permissions that recursively affect
	// future folders and files.
	switch os {
	case helperimage.OSTypeWindows:
		chmod := "icacls $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%q) /grant 'Everyone:(OI)(CI)F' /q | out-null"
		commands := []string{
			fmt.Sprintf(chmod, s.logsDir()),
			fmt.Sprintf(chmod, s.Build.RootDir),
		}
		container.Command = []string{s.Shell().Shell, "-c", strings.Join(commands, ";\n")}

	default:
		initCommand := fmt.Sprintf("touch %[1]s && (chmod 777 %[1]s || exit 0)", s.logFile())
		if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) &&
			s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) {
			initCommand = fmt.Sprintf("%s && cp /usr/bin/dumb-init %s", initCommand, s.scriptsDir())
		}
		container.Command = []string{"sh", "-c", initCommand}
	}

	return container, nil
}

func (s *executor) buildRedirectionCmd(shell string) string {
	switch shell {
	// powershell outputs utf16, so we re-encode the output to utf8
	// this is important because our json decoder that detects the exit status
	// of a job requires utf8.
	case shells.SNPowershell:
		return fmt.Sprintf("2>&1 | Out-File -Append -Encoding UTF8 %s", s.logFile())
	}

	return fmt.Sprintf("2>&1 | tee -a %s", s.logFile())
}

func (s *executor) processLogs(ctx context.Context) {
	processor := s.newLogProcessor()
	logsCh, errCh := processor.Process(ctx)

	for {
		select {
		case line, ok := <-logsCh:
			if !ok {
				return
			}
			s.forwardLogLine(line)
		case err, ok := <-errCh:
			if !ok {
				continue
			}

			if err != nil {
				s.Warningln(fmt.Sprintf("Error processing the log file: %v", err))
			}

			exitCode := getExitCode(err)
			// Script can be kept to nil as not being used after the exitStatus is received
			s.remoteProcessTerminated <- shells.StageCommandStatus{CommandExitCode: &exitCode}
		}
	}
}

func (s *executor) forwardLogLine(line string) {
	var status shells.StageCommandStatus
	if !status.TryUnmarshal(line) {
		if _, err := s.writeRunnerLog(line); err != nil {
			s.Warningln(fmt.Sprintf("Error writing log line to trace: %v", err))
		}

		return
	}

	s.Debugln(fmt.Sprintf("Setting remote stage status: %s", status))
	s.remoteStageStatusMutex.Lock()
	s.remoteStageStatus = status
	s.remoteStageStatusMutex.Unlock()

	if status.IsExited() {
		s.remoteProcessTerminated <- status
	}
}

func (s *executor) writeRunnerLog(log string) (int, error) {
	size := len(log)

	if size > common.DefaultReaderBufferSize && string(log[size-1]) == "\n" {
		log = log[:size-1]
	}

	return s.Trace.Write([]byte(log))
}

// getExitCode tries to extract the exit code from an inner exec.CodeExitError
// This error may be returned by the underlying kubernetes connection stream
// however it's not guaranteed to be.
// getExitCode would return unknownLogProcessorExitCode if err isn't of type exec.CodeExitError
// or if it's nil
func getExitCode(err error) int {
	var exitErr exec.CodeExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return unknownLogProcessorExitCode
}

func (s *executor) setupTrappingScripts(ctx context.Context) error {
	s.Debugln("Setting up trapping scripts on emptyDir ...")

	scriptName, script := "", ""
	shellName := s.Shell().Shell
	switch shellName {
	case shells.SNPwsh, shells.SNPowershell:
		scriptName, script = s.scriptName(pwshJSONTerminationScriptName), shells.PwshJSONTerminationScript(shellName)
	default:
		scriptName, script = s.scriptName(detectShellScriptName), shells.BashDetectShellScript
	}

	return s.saveScriptOnEmptyDir(ctx, scriptName, buildContainerName, script)
}

func (s *executor) saveScriptOnEmptyDir(ctx context.Context, scriptName, containerName, script string) error {
	shell, err := s.retrieveShell()
	if err != nil {
		return err
	}

	scriptPath := fmt.Sprintf("%s/%s", s.scriptsDir(), scriptName)
	saveScript, err := shell.GenerateSaveScript(*s.Shell(), scriptPath, script)
	if err != nil {
		return err
	}
	s.Debugln(fmt.Sprintf("Saving stage script %s on Container %q", saveScript, containerName))

	select {
	case err := <-s.runInContainerWithExec(
		ctx,
		containerName,
		s.BuildShell.DockerCommand,
		saveScript,
	):
		s.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: exitError.ExitStatus()}
		}
		return err

	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) retrieveShell() (common.Shell, error) {
	bashShell, ok := common.GetShell(s.Shell().Shell).(*shells.BashShell)
	if ok {
		return bashShell, nil
	}

	shell := common.GetShell(s.Shell().Shell)
	if shell == nil {
		return nil, errIncorrectShellType
	}

	return shell, nil
}

func (s *executor) Finish(err error) {
	if IsKubernetesPodNotFoundError(err) {
		// Avoid an additional error message when trying to
		// cleanup a pod that we know no longer exists
		s.pod = nil
	}

	s.AbstractExecutor.Finish(err)
}

func (s *executor) Cleanup() {
	if s.eventsStream != nil {
		s.eventsStream.Stop()
	}

	s.cleanupResources()
	closeKubeClient(s.kubeClient)
	s.AbstractExecutor.Cleanup()
}

// cleanupResources deletes the resources used during the runner job
// Having a pod does not mean that the owner-dependent relationship exists as an error may occur during setting
// We therefore explicitly delete the resources if no ownerReference is found on it
// This does not apply for services as they are created with the owner from the start
// thus deletion of the pod automatically means deletion of the services if any
func (s *executor) cleanupResources() {
	// Here we don't use the build context as its timeout will prevent a successful cleanup of the resources.
	// The solution used here is inspired from the one used for the docker executor.
	// We give a configurable timeout to complete the resources cleanup.
	ctx, cancel := context.WithTimeout(context.Background(), s.Config.Kubernetes.GetCleanupResourcesTimeout())
	defer cancel()

	if s.pod != nil {
		kubeRequest := newRetryableKubeAPICall(func() error {
			return s.kubeClient.CoreV1().
				Pods(s.pod.Namespace).
				Delete(ctx, s.pod.Name, metav1.DeleteOptions{
					GracePeriodSeconds: s.Config.Kubernetes.GetCleanupGracePeriodSeconds(),
					PropagationPolicy:  &PropagationPolicy,
				})
		})

		if err := kubeRequest.Run(); err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up pod: %s", err.Error()))
		}
	}

	if s.credentials != nil && len(s.credentials.OwnerReferences) == 0 {
		kubeRequest := newRetryableKubeAPICall(func() error {
			return s.kubeClient.CoreV1().
				Secrets(s.configurationOverwrites.namespace).
				Delete(ctx, s.credentials.Name, metav1.DeleteOptions{
					GracePeriodSeconds: s.Config.Kubernetes.GetCleanupGracePeriodSeconds(),
				})
		})
		if err := kubeRequest.Run(); err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up secrets: %s", err.Error()))
		}
	}
}

//nolint:funlen
func (s *executor) buildContainer(opts containerBuildOpts) (api.Container, error) {
	// check if the image/service is allowed
	internalImages := []string{
		s.ExpandValue(s.Config.Kubernetes.Image),
		s.ExpandValue(s.helperImageInfo.Name),
	}

	var (
		optionName    string
		allowedImages []string
		envVars       []common.JobVariable
	)
	if opts.isServiceContainer {
		optionName = "services"
		allowedImages = s.Config.Kubernetes.AllowedServices
		envVars = s.getServiceVariables(opts.imageDefinition)
	} else if opts.name == buildContainerName {
		optionName = "images"
		allowedImages = s.Config.Kubernetes.AllowedImages
		envVars = s.Build.GetAllVariables().PublicOrInternal()
	}

	verifyAllowedImageOptions := common.VerifyAllowedImageOptions{
		Image:          opts.image,
		OptionName:     optionName,
		AllowedImages:  allowedImages,
		InternalImages: internalImages,
	}
	err := common.VerifyAllowedImage(verifyAllowedImageOptions, s.BuildLogger)
	if err != nil {
		return api.Container{}, err
	}

	containerPorts := make([]api.ContainerPort, len(opts.imageDefinition.Ports))
	proxyPorts := make([]proxy.Port, len(opts.imageDefinition.Ports))

	for i, port := range opts.imageDefinition.Ports {
		proxyPorts[i] = proxy.Port{Name: port.Name, Number: port.Number, Protocol: port.Protocol}
		containerPorts[i] = api.ContainerPort{ContainerPort: int32(port.Number)}
	}

	if len(proxyPorts) > 0 {
		aliases := opts.imageDefinition.Aliases()
		if len(aliases) == 0 {
			if opts.name != buildContainerName {
				aliases = []string{fmt.Sprintf("proxy-%s", opts.name)}
			} else {
				aliases = []string{opts.name}
			}
		}

		for _, serviceName := range aliases {
			s.ProxyPool[serviceName] = s.newProxy(serviceName, proxyPorts)
		}
	}

	pullPolicy, err := s.pullManager.GetPullPolicyFor(opts.image)
	if err != nil {
		return api.Container{}, err
	}

	command, args := s.getCommandAndArgs(opts, opts.imageDefinition, opts.command...)

	container := api.Container{
		Name:            opts.name,
		Image:           opts.image,
		ImagePullPolicy: pullPolicy,
		Command:         command,
		Args:            args,
		Env:             buildVariables(envVars),
		Resources: api.ResourceRequirements{
			Limits:   opts.limits,
			Requests: opts.requests,
		},
		Ports:           containerPorts,
		VolumeMounts:    s.getVolumeMounts(),
		SecurityContext: opts.securityContext,
		Lifecycle:       s.prepareLifecycleHooks(),
		Stdin:           true,
	}

	return container, nil
}

func (s *executor) getCommandAndArgs(
	opts containerBuildOpts,
	imageDefinition common.Image,
	command ...string,
) ([]string, []string) {
	if s.Build.IsFeatureFlagOn(featureflags.KubernetesHonorEntrypoint) {
		return []string{}, command
	}

	var args []string
	cmd := command

	if len(lo.WithoutEmpty[string](imageDefinition.Entrypoint)) > 0 {
		switch {
		case opts.isServiceContainer:
			cmd = imageDefinition.Entrypoint
		default:
			cmd = imageDefinition.Entrypoint
			args = command
		}
	}

	if len(args) == 0 && len(imageDefinition.Command) > 0 {
		args = imageDefinition.Command
	}

	return cmd, args
}

func (s *executor) logFile() string {
	return path.Join(s.logsDir(), "output.log")
}

func (s *executor) logsDir() string {
	return fmt.Sprintf("/logs-%d-%d", s.Build.JobInfo.ProjectID, s.Build.JobResponse.ID)
}

func (s *executor) scriptsDir() string {
	return fmt.Sprintf("/scripts-%d-%d", s.Build.JobInfo.ProjectID, s.Build.JobResponse.ID)
}

func (s *executor) scriptPath(stage common.BuildStage) string {
	return path.Join(s.scriptsDir(), s.scriptName(string(stage)))
}

func (s *executor) scriptName(name string) string {
	shell := s.Shell()
	conf, err := common.GetShell(shell.Shell).GetConfiguration(*shell)
	if err != nil || conf.Extension == "" {
		return name
	}

	return name + "." + conf.Extension
}

func (s *executor) getVolumeMounts() []api.VolumeMount {
	var mounts []api.VolumeMount

	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		// These volume mounts **MUST NOT** be mounted inside another volume mount.
		// E.g. mounting them inside the "repo" volume mount will cause the whole volume
		// to be owned by root instead of the current user of the image. Something similar
		// is explained here https://github.com/kubernetes/kubernetes/issues/2630#issuecomment-64679120
		// where the first container determines the ownership of a volume. However, it seems like
		// when mounting a volume inside another volume the first container or the first point of contact
		// becomes root, regardless of SecurityContext or Image settings changing the user ID of the container.
		// This causes builds to stop working in environments such as OpenShift where there's no root access
		// resulting in an inability to modify anything inside the parent volume.
		mounts = append(
			mounts,
			api.VolumeMount{
				Name:      "scripts",
				MountPath: s.scriptsDir(),
			},
			api.VolumeMount{
				Name:      "logs",
				MountPath: s.logsDir(),
			})
	}

	mounts = append(mounts, s.getVolumeMountsForConfig()...)

	if s.isDefaultBuildsDirVolumeRequired() {
		mounts = append(mounts, api.VolumeMount{
			Name:      "repo",
			MountPath: s.AbstractExecutor.RootDir(),
		})
	}

	return mounts
}

func (s *executor) getVolumeMountsForConfig() []api.VolumeMount {
	var mounts []api.VolumeMount

	for _, mount := range s.Config.Kubernetes.Volumes.HostPaths {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.Secrets {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.PVCs {
		mounts = append(mounts, api.VolumeMount{
			Name:      s.Build.GetAllVariables().ExpandValue(mount.Name),
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.ConfigMaps {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.EmptyDirs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.CSIs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	return mounts
}

func (s *executor) getVolumes() []api.Volume {
	volumes := s.getVolumesForConfig()

	if s.isDefaultBuildsDirVolumeRequired() {
		volumes = append(volumes, api.Volume{
			Name: "repo",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		return volumes
	}

	volumes = append(
		volumes,
		api.Volume{
			Name: "scripts",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		},
		api.Volume{
			Name: "logs",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})

	return volumes
}

func (s *executor) getVolumesForConfig() []api.Volume {
	var volumes []api.Volume

	volumes = append(volumes, s.getVolumesForHostPaths()...)
	volumes = append(volumes, s.getVolumesForSecrets()...)
	volumes = append(volumes, s.getVolumesForPVCs()...)
	volumes = append(volumes, s.getVolumesForConfigMaps()...)
	volumes = append(volumes, s.getVolumesForEmptyDirs()...)
	volumes = append(volumes, s.getVolumesForCSIs()...)

	return volumes
}

func (s *executor) getVolumesForHostPaths() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.HostPaths {
		path := volume.HostPath
		// Make backward compatible with syntax introduced in version 9.3.0
		if path == "" {
			path = volume.MountPath
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: path,
				},
			},
		})
	}

	return volumes
}

func (s *executor) getVolumesForSecrets() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.Secrets {
		var items []api.KeyToPath
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				Secret: &api.SecretVolumeSource{
					SecretName: volume.Name,
					Items:      items,
				},
			},
		})
	}

	return volumes
}

func (s *executor) getVolumesForPVCs() []api.Volume {
	var volumes []api.Volume

	store := make(map[string]api.Volume)

	for _, volume := range s.Config.Kubernetes.Volumes.PVCs {
		if _, found := store[volume.Name]; found {
			continue
		}

		// Resolve the runtime name by injecting variable references.
		var resolvedName = s.Build.GetAllVariables().ExpandValue(volume.Name)

		apiVolume := api.Volume{
			Name: resolvedName,
			VolumeSource: api.VolumeSource{
				PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
					ClaimName: resolvedName,
				},
			},
		}

		volumes = append(volumes, apiVolume)
		store[volume.Name] = apiVolume
	}

	return volumes
}

func (s *executor) getVolumesForConfigMaps() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.ConfigMaps {
		var items []api.KeyToPath
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				ConfigMap: &api.ConfigMapVolumeSource{
					LocalObjectReference: api.LocalObjectReference{
						Name: volume.Name,
					},
					Items: items,
				},
			},
		})
	}

	return volumes
}

func (s *executor) getVolumesForEmptyDirs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.EmptyDirs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{
					Medium:    api.StorageMedium(volume.Medium),
					SizeLimit: s.parseVolumeSizeLimit(volume),
				},
			},
		})
	}
	return volumes
}

func (s *executor) parseVolumeSizeLimit(volume common.KubernetesEmptyDir) *resource.Quantity {
	quantity, err := resource.ParseQuantity(volume.SizeLimit)
	if err != nil {
		s.Warningln(fmt.Sprintf("invalid limit quantity %q for empty volume %q: %v", volume.SizeLimit, volume.Name, err))
		return nil
	}
	return &quantity
}

func (s *executor) getVolumesForCSIs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.CSIs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				CSI: &api.CSIVolumeSource{
					Driver:           volume.Driver,
					FSType:           &volume.FSType,
					ReadOnly:         &volume.ReadOnly,
					VolumeAttributes: volume.VolumeAttributes,
				},
			},
		})
	}
	return volumes
}

func (s *executor) isDefaultBuildsDirVolumeRequired() bool {
	if s.requireDefaultBuildsDirVolume != nil {
		return *s.requireDefaultBuildsDirVolume
	}

	var required = true
	for _, mount := range s.getVolumeMountsForConfig() {
		if mount.MountPath == s.AbstractExecutor.RootDir() {
			required = false
			break
		}
	}

	s.requireDefaultBuildsDirVolume = &required

	return required
}

func (s *executor) isSharedBuildsDirRequired() bool {
	// Return quickly when default builds dir is used as job is
	// isolated to pod, so no need for SharedBuildsDir behavior
	if s.isDefaultBuildsDirVolumeRequired() {
		return false
	}

	var required = true
	if s.requireSharedBuildsDir != nil {
		return *s.requireSharedBuildsDir
	}

	// Fetch name of the volume backing the builds volume mount
	buildVolumeName := "repo"
	for _, mount := range s.getVolumeMountsForConfig() {
		if mount.MountPath == s.AbstractExecutor.RootDir() {
			buildVolumeName = mount.Name
			break
		}
	}

	// Require shared builds dir when builds dir volume is anything except an emptyDir
	for _, volume := range s.getVolumes() {
		if volume.Name == buildVolumeName && volume.VolumeSource.EmptyDir != nil {
			required = false
			break
		}
	}

	s.requireSharedBuildsDir = &required
	return required
}

func (s *executor) setupCredentials(ctx context.Context) error {
	s.Debugln("Setting up secrets")

	authConfigs, err := auth.ResolveConfigs(s.Build.GetDockerAuthConfig(), s.Shell().User, s.Build.Credentials)
	if err != nil {
		return err
	}

	if len(authConfigs) == 0 {
		return nil
	}

	dockerCfgs := make(map[string]types.AuthConfig)
	for registry, registryInfo := range authConfigs {
		dockerCfgs[registry] = registryInfo.AuthConfig
	}

	dockerCfgContent, err := json.Marshal(dockerCfgs)
	if err != nil {
		return err
	}

	secret := api.Secret{}
	secret.Name = generateNameForK8sResources(s.Build.ProjectUniqueName())
	secret.Namespace = s.configurationOverwrites.namespace
	secret.Type = api.SecretTypeDockercfg
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigKey] = dockerCfgContent

	kubeRequest := newRetryableKubeAPICallWithValue(func() (*api.Secret, error) {
		return s.requestSecretCreation(ctx, &secret, s.configurationOverwrites.namespace)
	})
	s.credentials, err = kubeRequest.RunValue()
	return err
}

func (s *executor) requestSecretCreation(
	ctx context.Context,
	secret *api.Secret,
	namespace string,
) (*api.Secret, error) {
	creds, err := s.kubeClient.CoreV1().
		Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if isConflict(err) {
		s.Debugln(
			fmt.Sprintf(
				"Conflict while trying to create the secret  %s ... Retrieving the existing resource",
				secret.Name,
			),
		)

		creds, err = s.kubeClient.CoreV1().
			Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	}

	return creds, err
}

func (s *executor) getHostAliases() ([]api.HostAlias, error) {
	supportsHostAliases, err := s.featureChecker.IsHostAliasSupported()
	switch {
	case errors.Is(err, &badVersionError{}):
		s.Warningln("Checking for host alias support. Host aliases will be disabled.", err)
		return nil, nil
	case err != nil:
		return nil, err
	case !supportsHostAliases:
		return nil, nil
	}

	return createHostAliases(s.options.Services, s.Config.Kubernetes.GetHostAliases())
}

func (s *executor) setupBuildPod(ctx context.Context, initContainers []api.Container) error {
	s.Debugln("Setting up build pod")

	prepareOpts, err := s.createPodConfigPrepareOpts(initContainers)
	if err != nil {
		return err
	}

	podConfig, err := s.preparePodConfig(prepareOpts)
	if err != nil {
		return err
	}

	s.Debugln("Checking for ImagePullSecrets or ServiceAccount existence")
	err = s.checkDependantResources(ctx)
	if err != nil {
		return err
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseAdvancedPodSpecConfiguration) {
		s.Warningln("Advanced Pod Spec configuration enabled, merging the provided PodSpec to the generated one. " +
			"This is an alpha feature and is subject to change. Feedback is collected in this issue: " +
			"https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29659 ...")
		podConfig.Spec, err = s.applyPodSpecMerge(&podConfig.Spec)
		if err != nil {
			return err
		}
	}

	s.Debugln("Creating build pod")

	kubeRequest := newRetryableKubeAPICallWithValue(func() (*api.Pod, error) {
		return s.requestPodCreation(ctx, &podConfig, s.configurationOverwrites.namespace)
	})
	s.pod, err = kubeRequest.RunValue()
	if err != nil {
		return err
	}

	ownerReferences := s.buildPodReferences()
	err = s.setOwnerReferencesForResources(ctx, ownerReferences)
	if err != nil {
		return fmt.Errorf("error setting ownerReferences: %w", err)
	}

	s.services, err = s.makePodProxyServices(ctx, ownerReferences)
	return err
}

func (s *executor) requestPodCreation(ctx context.Context, pod *api.Pod, namespace string) (*api.Pod, error) {
	p, err := s.kubeClient.CoreV1().
		Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if isConflict(err) {
		s.Debugln(
			fmt.Sprintf(
				"Conflict while trying to create the pod  %s ... Retrieving the existing resource",
				pod.Name,
			),
		)

		p, err = s.kubeClient.CoreV1().
			Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	}

	return p, err
}

func (s *executor) checkDependantResources(ctx context.Context) error {
	if s.Config.Kubernetes.GetResourceAvailabilityCheckMaxAttempts() == 0 {
		s.Debugln("Resources check has been disabled")
		return nil
	}

	err := s.waitForResource(
		ctx,
		resourceTypeServiceAccount,
		s.Config.Kubernetes.ServiceAccount,
		s.serviceAccountExists(),
	)
	if err != nil {
		return err
	}

	for _, secretName := range s.Config.Kubernetes.ImagePullSecrets {
		err = s.waitForResource(
			ctx,
			resourceTypePullSecret,
			secretName,
			s.secretExists(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *executor) createPodConfigPrepareOpts(initContainers []api.Container) (podConfigPrepareOpts, error) {
	podServices, err := s.preparePodServices()
	if err != nil {
		return podConfigPrepareOpts{}, err
	}

	// We set a default label to the pod. This label will be used later
	// by the services, to link each service to the pod
	labels := map[string]string{"pod": s.Build.ProjectUniqueName()}
	for k, v := range s.Build.Runner.Kubernetes.PodLabels {
		labels[k] = sanitizeLabel(s.Build.Variables.ExpandValue(v))
	}
	for key, val := range s.configurationOverwrites.podLabels {
		labels[key] = sanitizeLabel(s.Build.Variables.ExpandValue(val))
	}

	annotations := map[string]string{
		"job." + k8sAnnotationPrefix + "id":         strconv.FormatInt(s.Build.ID, 10),
		"job." + k8sAnnotationPrefix + "url":        s.Build.JobURL(),
		"job." + k8sAnnotationPrefix + "sha":        s.Build.GitInfo.Sha,
		"job." + k8sAnnotationPrefix + "before_sha": s.Build.GitInfo.BeforeSha,
		"job." + k8sAnnotationPrefix + "ref":        s.Build.GitInfo.Ref,
		"job." + k8sAnnotationPrefix + "name":       s.Build.JobInfo.Name,
		"project." + k8sAnnotationPrefix + "id":     strconv.FormatInt(s.Build.JobInfo.ProjectID, 10),
	}
	for key, val := range s.configurationOverwrites.podAnnotations {
		annotations[key] = s.Build.Variables.ExpandValue(val)
	}

	imagePullSecrets := s.prepareImagePullSecrets()
	hostAliases, err := s.getHostAliases()
	if err != nil {
		return podConfigPrepareOpts{}, err
	}

	return podConfigPrepareOpts{
		labels:           labels,
		annotations:      annotations,
		services:         podServices,
		imagePullSecrets: imagePullSecrets,
		hostAliases:      hostAliases,
		initContainers:   initContainers,
	}, nil
}

func (s *executor) defaultCapDrop() []string {
	os := s.helperImageInfo.OSType
	// windows does not support security context capabilities
	if os == helperimage.OSTypeWindows {
		return nil
	}

	return []string{
		// Reasons for disabling NET_RAW by default were
		// discussed in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26833
		"NET_RAW",
	}
}

func (s *executor) prepareImagePullSecrets() []api.LocalObjectReference {
	var imagePullSecrets []api.LocalObjectReference
	for _, imagePullSecret := range s.Config.Kubernetes.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
	}

	if s.credentials != nil {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: s.credentials.Name})
	}

	return imagePullSecrets
}

func (s *executor) preparePodServices() ([]api.Container, error) {
	var err error
	podServices := make([]api.Container, len(s.options.Services))

	for i, service := range s.options.Services {
		podServices[i], err = s.buildContainer(containerBuildOpts{
			name:               fmt.Sprintf("%s%d", serviceContainerPrefix, i),
			image:              service.Name,
			imageDefinition:    service,
			isServiceContainer: true,
			requests:           s.configurationOverwrites.serviceRequests,
			limits:             s.configurationOverwrites.serviceLimits,
			securityContext: s.Config.Kubernetes.GetContainerSecurityContext(
				s.Config.Kubernetes.ServiceContainerSecurityContext,
				s.defaultCapDrop()...,
			),
		})
		if err != nil {
			return nil, err
		}
	}

	return podServices, nil
}

func (s *executor) preparePodConfig(opts podConfigPrepareOpts) (api.Pod, error) {
	buildContainer, helperContainer, err := s.createBuildAndHelperContainers()
	if err != nil {
		return api.Pod{}, err
	}

	pod := api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generateNameForK8sResources(s.Build.ProjectUniqueName()),
			Namespace:   s.configurationOverwrites.namespace,
			Labels:      opts.labels,
			Annotations: opts.annotations,
		},
		Spec: api.PodSpec{
			Volumes:            s.getVolumes(),
			SchedulerName:      s.Config.Kubernetes.SchedulerName,
			ServiceAccountName: s.configurationOverwrites.serviceAccount,
			RestartPolicy:      api.RestartPolicyNever,
			NodeSelector:       s.configurationOverwrites.nodeSelector,
			Tolerations:        s.Config.Kubernetes.GetNodeTolerations(),
			InitContainers:     opts.initContainers,
			Containers: append([]api.Container{
				buildContainer,
				helperContainer,
			}, opts.services...),
			TerminationGracePeriodSeconds: s.Config.Kubernetes.GetPodTerminationGracePeriodSeconds(),
			ActiveDeadlineSeconds:         s.getPodActiveDeadlineSeconds(),
			ImagePullSecrets:              opts.imagePullSecrets,
			SecurityContext:               s.Config.Kubernetes.GetPodSecurityContext(),
			HostAliases:                   opts.hostAliases,
			Affinity:                      s.Config.Kubernetes.GetAffinity(),
			DNSPolicy:                     s.getDNSPolicy(),
			DNSConfig:                     s.Config.Kubernetes.GetDNSConfig(),
			RuntimeClassName:              s.Config.Kubernetes.RuntimeClassName,
			PriorityClassName:             s.Config.Kubernetes.PriorityClassName,
		},
	}

	return pod, nil
}

// getPodActiveDeadlineSeconds returns the effective build/job timeout
// The feature is behind a FF and return a nil pointer when
// FF_POD_ACTIVE_DEADLINE_SECONDS is disabled
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29279.
func (s *executor) getPodActiveDeadlineSeconds() *int64 {
	if !s.Build.IsFeatureFlagOn(featureflags.UsePodActiveDeadlineSeconds) {
		return nil
	}

	s.Println(fmt.Sprintf(
		"Using FF_USE_POD_ACTIVE_DEADLINE_SECONDS, the Pod activeDeadlineSeconds will be set to the job timeout: %v...",
		time.Duration(s.Build.RunnerInfo.Timeout)*time.Second,
	))

	// We do not set the exact timeout as activeDeadlineSeconds
	// 1 second is added to allow the job to timeout on GitLab side
	// before the pod can be marked as failed and the container killed
	timeout := int64(s.Build.RunnerInfo.Timeout + 1)

	return &timeout
}

func (s *executor) createBuildAndHelperContainers() (api.Container, api.Container, error) {
	buildContainer, err := s.buildContainer(containerBuildOpts{
		name:            buildContainerName,
		image:           s.options.Image.Name,
		imageDefinition: s.options.Image,
		requests:        s.configurationOverwrites.buildRequests,
		limits:          s.configurationOverwrites.buildLimits,
		securityContext: s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.BuildContainerSecurityContext,
			s.defaultCapDrop()...,
		),
		command: s.getBuildAndHelperContainersCommand(),
	})
	if err != nil {
		return api.Container{}, api.Container{}, fmt.Errorf("building build container: %w", err)
	}

	helperContainer, err := s.buildContainer(containerBuildOpts{
		name:     helperContainerName,
		image:    s.getHelperImage(),
		requests: s.configurationOverwrites.helperRequests,
		limits:   s.configurationOverwrites.helperLimits,
		securityContext: s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.HelperContainerSecurityContext,
			s.defaultCapDrop()...,
		),
		command: s.getBuildAndHelperContainersCommand(),
	})
	if err != nil {
		return api.Container{}, api.Container{}, fmt.Errorf("building helper container: %w", err)
	}

	return buildContainer, helperContainer, nil
}

func (s *executor) getBuildAndHelperContainersCommand() []string {
	if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) ||
		!s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) {
		return s.BuildShell.DockerCommand
	}

	switch s.Shell().Shell {
	case shells.SNPowershell:
		return s.BuildShell.DockerCommand
	default:
		return append([]string{fmt.Sprintf("%s/dumb-init", s.scriptsDir()), "--"}, s.BuildShell.DockerCommand...)
	}
}

// Inspired by
// https://github.com/kubernetes/kubernetes/blob/cde45fb161c5a4bfa7cfe45dfd814f6cc95433f7/cmd/kubeadm/app/util/patches/patches.go#L171
func (s *executor) applyPodSpecMerge(podSpec *api.PodSpec) (api.PodSpec, error) {
	patchedData, err := json.Marshal(podSpec)
	if err != nil {
		return api.PodSpec{}, err
	}

	for _, spec := range s.Config.Kubernetes.PodSpec {
		patchedData, err = doPodSpecMerge(patchedData, spec)
		if err != nil {
			return api.PodSpec{}, err
		}
	}

	var patchedPodSpec api.PodSpec
	err = json.Unmarshal(patchedData, &patchedPodSpec)
	return patchedPodSpec, err
}

func doPodSpecMerge(original []byte, spec common.KubernetesPodSpec) ([]byte, error) {
	var data []byte

	patchBytes, patchType, err := spec.PodSpecPatch()
	if err != nil {
		return nil, err
	}

	switch patchType {
	case common.PatchTypeJSONPatchType:
		var patchObj jsonpatch.Patch
		patchObj, err = jsonpatch.DecodePatch(patchBytes)
		if err == nil {
			data, err = patchObj.Apply(original)
		}

		if err != nil {
			return nil, err
		}
	case common.PatchTypeMergePatchType:
		data, err = jsonpatch.MergePatch(original, patchBytes)
		if err != nil {
			return nil, err
		}
	case common.PatchTypeStrategicMergePatchType:
		data, err = strategicpatch.StrategicMergePatch(
			original,
			patchBytes,
			api.PodSpec{},
		)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported patch type %v", patchType)
	}

	return data, nil
}

func (s *executor) setOwnerReferencesForResources(ctx context.Context, ownerReferences []metav1.OwnerReference) error {
	if s.credentials == nil {
		return nil
	}

	kubeRequest := newRetryableKubeAPICallWithValue(func() (*api.Secret, error) {
		credentials := s.credentials.DeepCopy()
		credentials.SetOwnerReferences(ownerReferences)

		return s.kubeClient.CoreV1().
			Secrets(s.configurationOverwrites.namespace).
			Update(ctx, credentials, metav1.UpdateOptions{})
	})
	var err error
	s.credentials, err = kubeRequest.RunValue()
	return err
}

func (s *executor) buildPodReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: apiVersion,
			Kind:       ownerReferenceKind,
			Name:       s.pod.GetName(),
			UID:        s.pod.GetUID(),
		},
	}
}

func (s *executor) waitForResource(
	ctx context.Context,
	resourceType string,
	resourceName string,
	checkExists func(context.Context, string) bool,
) error {
	attempt := -1

	s.Debugln(fmt.Sprintf("Checking for %s existence", resourceType))

	for attempt < s.Config.Kubernetes.GetResourceAvailabilityCheckMaxAttempts() {
		if checkExists(ctx, resourceName) {
			return nil
		}

		attempt++
		if attempt > 0 {
			s.Debugln(fmt.Sprintf(
				"Pausing check of the %s availability for %d (attempt %d)",
				resourceType,
				resourceAvailabilityCheckMaxPollInterval,
				attempt,
			))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(resourceAvailabilityCheckMaxPollInterval):
		}
	}

	return &resourceCheckError{
		resourceType: resourceType,
		resourceName: resourceName,
	}
}

func (s *executor) serviceAccountExists() func(context.Context, string) bool {
	return func(ctx context.Context, saName string) bool {
		if saName == "" {
			return true
		}

		kubeRequest := newRetryableKubeAPICall(func() error {
			_, err := s.kubeClient.CoreV1().
				ServiceAccounts(s.configurationOverwrites.namespace).Get(ctx, saName, metav1.GetOptions{})
			return err
		})

		return kubeRequest.Run() == nil
	}
}

func (s *executor) secretExists() func(context.Context, string) bool {
	return func(ctx context.Context, secretName string) bool {
		kubeRequest := newRetryableKubeAPICall(func() error {
			_, err := s.kubeClient.CoreV1().
				Secrets(s.configurationOverwrites.namespace).Get(ctx, secretName, metav1.GetOptions{})
			return err
		})

		return kubeRequest.Run() == nil
	}
}

func (s *executor) getDNSPolicy() api.DNSPolicy {
	dnsPolicy, err := s.Config.Kubernetes.DNSPolicy.Get()
	if err != nil {
		s.Warningln(fmt.Sprintf("falling back to cluster's default policy: %v", err))
	}
	return dnsPolicy
}

func (s *executor) getHelperImage() string {
	if len(s.Config.Kubernetes.HelperImage) > 0 {
		return s.ExpandValue(s.Config.Kubernetes.HelperImage)
	}

	return s.helperImageInfo.String()
}

func (s *executor) makePodProxyServices(
	ctx context.Context,
	ownerReferences []metav1.OwnerReference,
) ([]api.Service, error) {
	s.Debugln("Creating pod proxy services")

	ch := make(chan serviceCreateResponse)
	var wg sync.WaitGroup
	wg.Add(len(s.ProxyPool))

	for serviceName, serviceProxy := range s.ProxyPool {
		serviceName = dns.MakeRFC1123Compatible(serviceName)
		servicePorts := make([]api.ServicePort, len(serviceProxy.Settings.Ports))
		for i, port := range serviceProxy.Settings.Ports {
			// When there is more than one port Kubernetes requires a port name
			portName := fmt.Sprintf("%s-%d", serviceName, port.Number)
			servicePorts[i] = api.ServicePort{
				Port:       int32(port.Number),
				TargetPort: intstr.FromInt(port.Number),
				Name:       portName,
			}
		}

		serviceConfig := s.prepareServiceConfig(serviceName, servicePorts, ownerReferences)
		go s.createKubernetesService(ctx, &serviceConfig, serviceProxy.Settings, ch, &wg)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var proxyServices []api.Service
	for res := range ch {
		if res.err != nil {
			err := fmt.Errorf("error creating the proxy service %q: %w", res.service.Name, res.err)
			s.Errorln(err)

			return []api.Service{}, err
		}

		proxyServices = append(proxyServices, *res.service)
	}

	return proxyServices, nil
}

func (s *executor) prepareServiceConfig(
	name string,
	ports []api.ServicePort,
	ownerReferences []metav1.OwnerReference,
) api.Service {
	return api.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            generateNameForK8sResources(name),
			Namespace:       s.configurationOverwrites.namespace,
			OwnerReferences: ownerReferences,
		},
		Spec: api.ServiceSpec{
			Ports:    ports,
			Selector: map[string]string{"pod": s.Build.ProjectUniqueName()},
			Type:     api.ServiceTypeClusterIP,
		},
	}
}

func (s *executor) createKubernetesService(
	ctx context.Context,
	service *api.Service,
	proxySettings *proxy.Settings,
	ch chan<- serviceCreateResponse,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	kubeRequest := newRetryableKubeAPICallWithValue(func() (*api.Service, error) {
		return s.requestServiceCreation(ctx, service, s.pod.Namespace)
	})
	var err error
	service, err = kubeRequest.RunValue()
	if err == nil {
		// Updating the internal service name reference and activating the proxy
		proxySettings.ServiceName = service.Name
	}

	ch <- serviceCreateResponse{service: service, err: err}
}

func (s *executor) requestServiceCreation(
	ctx context.Context,
	service *api.Service,
	namespace string,
) (*api.Service, error) {
	srv, err := s.kubeClient.CoreV1().
		Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if isConflict(err) {
		s.Debugln(
			fmt.Sprintf(
				"Conflict while trying to create the service  %s ... Retrieving the existing resource",
				service.Name,
			),
		)

		srv, err = s.kubeClient.CoreV1().
			Services(namespace).Get(ctx, service.Name, metav1.GetOptions{})
	}

	return srv, err
}

func (s *executor) watchPodStatus(ctx context.Context) <-chan error {
	// Buffer of 1 in case the context is cancelled while the timer tick case is being executed
	// and the consumer is no longer reading from the channel while we try to write to it
	ch := make(chan error, 1)

	go func() {
		defer close(ch)

		t := time.NewTicker(time.Duration(s.Config.Kubernetes.GetPollInterval()) * time.Second)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				err := s.checkPodStatus(ctx)
				if err != nil {
					ch <- err
					return
				}
			}
		}
	}()

	return ch
}

func (s *executor) checkPodStatus(ctx context.Context) error {
	kubeRequest := newRetryableKubeAPICallWithValue(func() (*api.Pod, error) {
		return s.kubeClient.CoreV1().
			Pods(s.pod.Namespace).Get(ctx, s.pod.Name, metav1.GetOptions{})
	})

	pod, err := kubeRequest.RunValue()
	if IsKubernetesPodNotFoundError(err) {
		return err
	}

	if err != nil {
		// General request failure
		s.Warningln("Getting job pod status", err)
		return nil
	}

	if pod.Status.Phase != api.PodRunning {
		return &podPhaseError{
			name:  s.pod.Name,
			phase: pod.Status.Phase,
		}
	}

	return nil
}

func (s *executor) runInContainer(
	ctx context.Context,
	stage common.BuildStage,
	name string,
	command []string,
) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		attach := AttachOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,

			Config:   s.kubeConfig,
			Client:   s.kubeClient,
			Executor: &DefaultRemoteExecutor{},

			Context: ctx,
		}

		kubeRequest := newRetryableKubeAPICall(func() error {
			err := attach.Run()
			s.Debugln(fmt.Sprintf("Trying to execute stage %v, got error %v", stage, err))
			return s.checkScriptExecution(stage, err)
		})

		if err := kubeRequest.Run(); err != nil {
			errCh <- err
		}

		exitStatus := <-s.remoteProcessTerminated
		s.Debugln("Remote process exited with the status:", exitStatus)

		// CommandExitCode is guaranteed to be non nil when sent over the remoteProcessTerminated channel
		if *exitStatus.CommandExitCode == 0 {
			errCh <- nil
			return
		}

		errCh <- &commandTerminatedError{exitCode: *exitStatus.CommandExitCode}
	}()

	return errCh
}

func (s *executor) runInContainerWithExec(
	ctx context.Context,
	name string,
	command []string,
	script string,
) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		var out io.Writer = s.Trace
		if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
			out = io.Discard
		}

		status, err := waitForPodRunning(ctx, s.kubeClient, s.pod, out, s.Config.Kubernetes)
		if err != nil {
			errCh <- err
			return
		}

		if status != api.PodRunning {
			errCh <- fmt.Errorf("pod failed to enter running state: %s", status)
			return
		}

		exec := ExecOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,
			In:            strings.NewReader(script),
			Out:           s.Trace,
			Err:           s.Trace,
			Stdin:         true,
			Config:        s.kubeConfig,
			Client:        s.kubeClient,
			Executor:      &DefaultRemoteExecutor{},

			Context: ctx,
		}

		kubeRequest := newRetryableKubeAPICallWithValue(func() (any, error) {
			return nil, exec.Run()
		})
		errCh <- kubeRequest.Run()
	}()

	return errCh
}

func (s *executor) checkScriptExecution(stage common.BuildStage, err error) error {
	// Retrying attach command is a bit different from regular Kubernetes requests.
	// Since the attach commands are executed by openning an HTTP stream to the Kubernetes server
	// and piping the command into that stream and then expecting a response there's no good place to check
	// whether the whole command execution was successful.
	// If we check whether the Stdin stream was read - the connection might have broken up after during transit of that
	// meaning that the command was never executed.
	// It could have also been broken during the reading of the response stream - meaning that it was executed, but we can't know that.
	// The only solution is to check for certain whether the process is already running.
	// For attach that is easy since the process is completely running in the background, and we receive the status of it through
	// the log file and the log processor moves things forward.

	// Non-network errors don't concern this function
	if !isNetworkError(err) {
		return err
	}

	s.remoteStageStatusMutex.Lock()
	defer s.remoteStageStatusMutex.Unlock()
	s.Debugln(fmt.Sprintf("Checking remote stage status after trying attach with err %v. Remote stage status: %v", err, s.remoteStageStatus))

	// If the remote stage is the one we are trying to retry it means that it was already executed.
	s.Debugln(fmt.Sprintf("Remote stage: %v, trying to execute stage %v", s.remoteStageStatus.BuildStage(), stage))
	if s.remoteStageStatus.BuildStage() == stage {
		return nil
	}

	// If the remote stage is not the same, then we can retry
	return err
}

func (s *executor) prepareOverwrites(variables common.JobVariables) error {
	values, err := createOverwrites(s.Config.Kubernetes, variables, s.BuildLogger)
	if err != nil {
		return err
	}

	s.configurationOverwrites = values
	return nil
}

func (s *executor) prepareOptions(build *common.Build) {
	s.options = &kubernetesOptions{}
	s.options.Image = build.Image

	s.getServices(build)
}

func (s *executor) prepareLifecycleHooks() *api.Lifecycle {
	lifecycleCfg := s.Config.Kubernetes.GetContainerLifecycle()

	if lifecycleCfg.PostStart == nil && lifecycleCfg.PreStop == nil {
		return nil
	}

	lifecycle := &api.Lifecycle{}

	if lifecycleCfg.PostStart != nil {
		lifecycle.PostStart = lifecycleCfg.PostStart.ToKubernetesLifecycleHandler()
	}
	if lifecycleCfg.PreStop != nil {
		lifecycle.PreStop = lifecycleCfg.PreStop.ToKubernetesLifecycleHandler()
	}

	return lifecycle
}

func (s *executor) getServices(build *common.Build) {
	for _, service := range s.Config.Kubernetes.GetExpandedServices(s.Build.GetAllVariables()) {
		if service.Name == "" {
			continue
		}
		s.options.Services = append(s.options.Services, service.ToImageDefinition())
	}

	for _, service := range build.Services {
		if service.Name == "" {
			continue
		}
		s.options.Services = append(s.options.Services, service)
	}
}

func (s *executor) getServiceVariables(serviceDefinition common.Image) common.JobVariables {
	variables := s.Build.GetAllVariables().PublicOrInternal()
	variables = append(variables, serviceDefinition.Variables...)

	return variables.Expand()
}

// checkDefaults Defines the configuration for the Pod on Kubernetes
func (s *executor) checkDefaults() error {
	k8sConfigImageName := s.ExpandValue(s.Config.Kubernetes.Image)
	if s.options.Image.Name == "" {
		if k8sConfigImageName == "" {
			return fmt.Errorf("no image specified and no default set in config")
		}

		s.options.Image = common.Image{
			Name: k8sConfigImageName,
		}
	}

	if s.configurationOverwrites.namespace == "" {
		s.Warningln(
			fmt.Sprintf("Namespace is empty, therefore assuming '%s'.", DefaultResourceIdentifier),
		)
		s.configurationOverwrites.namespace = DefaultResourceIdentifier
	}

	s.Println("Using Kubernetes namespace:", s.configurationOverwrites.namespace)

	return nil
}

// captureContainersLogs initiates capturing logs for the specified kubernetes
// managed containers to a desired additional sink. The sink can be any
// io.Writer. Currently the sink is the jobs main trace, which is wrapped in an
// inlineServiceLogWriter instance to add additional context to logs. In the
// future this could be separate file.
func (s *executor) captureContainersLogs(ctx context.Context, containers []api.Container) {
	if !s.Build.IsCIDebugServiceEnabled() {
		return
	}

	for _, service := range s.options.Services {
		for _, container := range containers {
			if service.Name != container.Image {
				continue
			}
			aliases := append([]string{strings.Split(container.Image, ":")[0]}, service.Aliases()...)
			sink := service_helpers.NewInlineServiceLogWriter(strings.Join(aliases, "-"), s.Trace)
			if err := s.captureContainerLogs(ctx, container.Name, sink); err != nil {
				s.Warningln(err.Error())
			}
		}
	}
}

// captureContainerLogs tails (i.e. reads) logs emitted to stdout or stdin from
// processes in the specified kubernetes managed container, and redirects them
// to the specified sink, which can be any io.Writer (e.g. this process's
// stdout, a file, a log aggregator). The logs are streamed as they are emitted,
// rather than batched and written when we disconnect from the container (or it
// is stopped). The specified sink is closed when the source is completely
// drained.
func (s *executor) captureContainerLogs(ctx context.Context, containerName string, sink io.WriteCloser) error {
	podLogOpts := api.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		Timestamps: true,
	}

	kubeRequest := newRetryableKubeAPICallWithValue(func() (io.ReadCloser, error) {
		return s.kubeClient.CoreV1().
			Pods(s.pod.Namespace).GetLogs(s.pod.Name, &podLogOpts).Stream(ctx)
	})

	podLogs, err := kubeRequest.RunValue()
	if err != nil {
		return fmt.Errorf("failed to open log stream for container %s: %w", containerName, err)
	}

	s.Debugln("streaming logs for container " + containerName)
	go func() {
		defer podLogs.Close()
		defer sink.Close()

		if _, err = io.Copy(sink, podLogs); err != nil {
			if err != io.EOF && !errors.Is(err, context.Canceled) {
				s.Warningln(fmt.Sprintf("error streaming logs for container %s: %s", containerName, err.Error()))
			}
		}
		s.Debugln("stopped streaming logs for container " + containerName)
	}()
	return nil
}

func generateNameForK8sResources(pattern string) string {
	suffix := make([]rune, k8sResourcesNameSuffixLength)
	for i := range suffix {
		suffix[i] = chars[rand.Intn(len(chars))]
	}

	if len(pattern) > (k8sResourcesNameMaxLength - k8sResourcesNameSuffixLength - 1) {
		pattern = pattern[:k8sResourcesNameMaxLength-k8sResourcesNameSuffixLength-1]
	}

	return fmt.Sprintf("%s-%s", pattern, string(suffix))
}

// When calling the k8s API request, it can happen that despite the failure of the request,
// the resource was actually created. When it comes to POST method, the following retries will get
// a 409 status code (conflits because of the name that must be unique)
// When such status code is received, we stop the retries
func isConflict(err error) bool {
	var statusError *kubeerrors.StatusError
	return errors.As(err, &statusError) &&
		statusError.ErrStatus.Code == http.StatusConflict &&
		strings.Contains(statusError.ErrStatus.Message, errorAlreadyExistsMessage)
}

func IsKubernetesPodNotFoundError(err error) bool {
	var statusErr *kubeerrors.StatusError
	return errors.As(err, &statusErr) &&
		statusErr.ErrStatus.Code == http.StatusNotFound &&
		statusErr.ErrStatus.Details != nil &&
		statusErr.ErrStatus.Details.Kind == "pods"
}

func newExecutor() *executor {
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
		remoteProcessTerminated: make(chan shells.StageCommandStatus),
	}

	e.newLogProcessor = func() logProcessor {
		return newKubernetesLogProcessor(
			e.kubeClient,
			e.kubeConfig,
			&backoff.Backoff{Min: time.Second, Max: 30 * time.Second},
			e.Build.Log(),
			kubernetesLogProcessorPodConfig{
				namespace:          e.pod.Namespace,
				pod:                e.pod.Name,
				container:          helperContainerName,
				logPath:            e.logFile(),
				waitLogFileTimeout: waitLogFileTimeout,
			},
		)
	}

	return e
}

func featuresFn(features *common.FeaturesInfo) {
	features.Variables = true
	features.Image = true
	features.Services = true
	features.Artifacts = true
	features.Cache = true
	features.FallbackCacheKeys = true
	features.Session = true
	features.Terminal = true
	features.Proxy = true
	features.ServiceVariables = true
	features.ServiceMultipleAliases = true
}

func init() {
	common.RegisterExecutorProvider(common.ExecutorKubernetes, executors.DefaultExecutorProvider{
		Creator: func() common.Executor {
			return newExecutor()
		},
		FeaturesUpdater:  featuresFn,
		DefaultShellName: executorOptions.Shell.Shell,
	})
}
