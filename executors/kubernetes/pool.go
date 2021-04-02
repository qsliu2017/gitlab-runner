package kubernetes

import api "k8s.io/api/core/v1"

type buildResources struct {
	pod         *api.Pod
	configMap   *api.ConfigMap
	credentials *api.Secret
	services    []api.Service
}

type buildResourcesCreatorFn func() (*buildResources, error)

type podsPool struct {
	idleCount int
	idleTime  int

	creator buildResourcesCreatorFn
}

func (p *podsPool) loop() {

}
