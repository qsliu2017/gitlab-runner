package kubernetes

import (
	"encoding/json"
	"fmt"
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

type buildResources struct {
	pod         *api.Pod
	configMap   *api.ConfigMap
	credentials *api.Secret
	services    []api.Service
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

func (p *podsManager) labels() map[string]string {
	return map[string]string{
		"gitlab-runner-idle":            "true",
		"gitlab-runner-idle-identifier": p.opts.config.ShortDescription(),
	}
}

func (p *podsManager) createIdle(e *executor) {
	p.RLock()
	if len(p.resources) >= p.opts.config.Kubernetes.IdleCount {
		p.RUnlock()
		return
	}
	p.RUnlock()

	list, err := p.kubeClient.CoreV1().Pods(p.opts.config.Kubernetes.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(p.labels()).String(),
	})
	if err != nil {
		fmt.Println("Listing pods ", err)
		return
	}

	for i := p.opts.config.Kubernetes.IdleCount - len(list.Items); i < p.opts.config.Kubernetes.IdleCount; i++ {
		resources, err := p.opts.creator(e, buildResourceOptions{
			labels: p.labels(),
		})
		if err != nil {
			fmt.Println("creating ", err)
			return
		}

		p.Lock()
		p.resources[resources.pod.Name] = resources
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

	managerLabels := p.labels()
	patches := make([]patchRemoveValue, 0, len(managerLabels))
	for k := range managerLabels {
		patches = append(patches, newPatchRemoveValue(k))
	}
	payload, _ := json.Marshal(patches)

	_, err := p.kubeClient.CoreV1().Pods(br.pod.Namespace).Patch(br.pod.Name, types.JSONPatchType, payload)
	if err != nil {
		fmt.Println("patching managerLabels ", err)
		return nil, err
	}

	return br, nil
}
