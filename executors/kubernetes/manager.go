package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/client-go/kubernetes"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	api "k8s.io/api/core/v1"
)

type patchRemoveValue struct {
	Op   string `json:"op"`
	Path string `json:"path"`
}

func newPatchRemoveValue(path string) patchRemoveValue {
	return patchRemoveValue{Op: "remove", Path: path}
}

type patchValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func newPatchValue(op, path, value string) patchValue {
	return patchValue{Op: op, Path: path, Value: value}
}

type buildResources struct {
	pod         *api.Pod
	configMap   *api.ConfigMap
	credentials *api.Secret
	services    []api.Service
}

func (b *buildResources) toLabels() map[string]string {
	var services []string
	for _, s := range b.services {
		services = append(services, s.Name)
	}

	return map[string]string{
		"gitlab-runner-idle-pod":         b.pod.Name,
		"gitlab-runner-idle-configMap":   b.configMap.Name,
		"gitlab-runner-idle-credentials": b.credentials.Name,
		"gitlab-runner-idle-services":    strings.Join(services, ","),
	}
}

func (b *buildResources) fromLabels(labels map[string]string) {
	b.pod = &api.Pod{}
	b.pod.Name = labels["gitlab-runner-idle-pod"]

	b.configMap = &api.ConfigMap{}
	b.configMap.Name = labels["gitlab-runner-idle-configMap"]

	b.credentials = &api.Secret{}
	b.credentials.Name = labels["gitlab-runner-idle-credentials"]

	for _, s := range strings.Split(labels["gitlab-runner-idle-services"], ",") {
		service := api.Service{}
		service.Name = s
		b.services = append(b.services, service)
	}
}

type buildResourceOptions struct {
	labels map[string]string
}

type buildResourcesCreatorFn func(*executor, buildResourceOptions) (*buildResources, error)
type buildResourcesCleanerFn func(*executor, *buildResources)

type podsManagerOptions struct {
	provider common.ExecutorProvider

	config  common.RunnerConfig
	creator buildResourcesCreatorFn
	cleaner buildResourcesCleanerFn
}

type podsManager struct {
	sync.RWMutex
	opts      *podsManagerOptions
	resources map[string]*buildResources

	kubeClient *kubernetes.Clientset
}

func newPodsManager(opts *podsManagerOptions) *podsManager {
	kubeConfig, err := getKubeClientConfig(opts.config.Kubernetes, &overwrites{})
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic(err)
	}

	return &podsManager{
		opts:       opts,
		kubeClient: kubeClient,
		resources:  map[string]*buildResources{},
	}
}

var podsManagerStorageLock sync.RWMutex
var podsManagersStorage = map[string]*podsManager{}

func getPodsManager(opts *podsManagerOptions) *podsManager {
	podsManagerStorageLock.Lock()
	defer podsManagerStorageLock.Unlock()

	identifier := opts.config.ShortDescription()
	if manager, ok := podsManagersStorage[identifier]; ok {
		manager.opts = opts
		return manager
	}

	manager := newPodsManager(opts)
	podsManagersStorage[identifier] = manager
	return manager
}

func (p *podsManager) setOptions(opts *podsManagerOptions) {
	p.Lock()
	defer p.Unlock()
	p.opts = opts
}

func (p *podsManager) labels(br *buildResources) map[string]string {
	podsLabels := map[string]string{
		"gitlab-runner-idle":            "true",
		"gitlab-runner-idle-identifier": p.opts.config.ShortDescription(),
		"gitlab-runner-idle-time":       fmt.Sprint(p.opts.config.Kubernetes.IdleTime),
	}

	if br != nil {
		for k, v := range br.toLabels() {
			podsLabels[k] = v
		}
	}

	return podsLabels
}

func (p *podsManager) needsMorePods() bool {
	p.RLock()
	defer p.RUnlock()
	if len(p.resources) >= p.opts.config.Kubernetes.IdleCount {
		return false
	}

	return true
}

func (p *podsManager) createIdle(e *executor) {
	if !p.needsMorePods() {
		return
	}

	list, err := p.kubeClient.CoreV1().Pods(p.opts.config.Kubernetes.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(p.labels(nil)).String(),
	})
	if err != nil {
		fmt.Println("Listing pods ", err)
		return
	}

	for _, pod := range list.Items {
		p.RLock()
		if _, ok := p.resources[pod.Name]; ok {
			p.RUnlock()
			continue
		}
		p.RUnlock()

		if _, ok := pod.Labels["gitlab-runner-idle-pod"]; !ok {
			//invalid
			continue
		}

		br := &buildResources{}
		br.fromLabels(pod.Labels)

		br.pod = &pod
		br.credentials, _ = p.kubeClient.CoreV1().Secrets(pod.Namespace).Get(br.credentials.Name, metav1.GetOptions{})
		br.configMap, _ = p.kubeClient.CoreV1().ConfigMaps(pod.Namespace).Get(br.configMap.Name, metav1.GetOptions{})

		for i, s := range br.services {
			service, _ := p.kubeClient.CoreV1().Services(pod.Namespace).Get(s.Name, metav1.GetOptions{})
			br.services[i] = *service
		}

		p.Lock()
		p.resources[pod.Name] = br
		p.Unlock()
	}

	if !p.needsMorePods() {
		return
	}

	p.RLock()
	has := len(p.resources)
	p.RUnlock()

	for i := has; i < p.opts.config.Kubernetes.IdleCount; i++ {
		resources, err := p.opts.creator(e, buildResourceOptions{
			labels: p.labels(nil),
		})
		if err != nil {
			fmt.Println("creating ", err)
			return
		}

		managerLabels := p.labels(resources)
		patches := make([]patchValue, 0)
		for k, v := range managerLabels {
			patches = append(patches, newPatchValue("add", fmt.Sprintf("/metadata/labels/%s", k), v))
		}
		payload, _ := json.Marshal(patches)

		_, err = p.kubeClient.CoreV1().Pods(resources.pod.Namespace).Patch(resources.pod.Name, types.JSONPatchType, payload)
		if err != nil {
			fmt.Println("adding resources label to pod")
			continue
		}

		p.Lock()
		p.resources[resources.pod.Name] = &*resources
		p.Unlock()
	}
}

func (p *podsManager) get(e *executor) (*buildResources, error) {
	p.createIdle(e)

	p.Lock()
	var br *buildResources
	for _, v := range p.resources {
		br = v
		break
	}
	if br != nil {
		delete(p.resources, br.pod.Name)
	}
	p.Unlock()

	managerLabels := p.labels(br)
	patches := make([]patchRemoveValue, 0, len(managerLabels))
	for k := range managerLabels {
		patches = append(patches, newPatchRemoveValue(fmt.Sprintf("/metadata/labels/%s", k)))
	}
	payload, _ := json.Marshal(patches)

	_, err := p.kubeClient.CoreV1().Pods(br.pod.Namespace).Patch(br.pod.Name, types.JSONPatchType, payload)
	if err != nil {
		fmt.Println("patching managerLabels ", err)
		return nil, err
	}

	go p.createIdle(e)
	return br, nil
}
