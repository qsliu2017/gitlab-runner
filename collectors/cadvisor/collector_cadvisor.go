package cadvisor

import (
	"fmt"
	"io"
	"net/http"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type cAdvisorCollector struct {
	containerName string
	ipAddress     string
	port          int
	url           string
}

func (c *cAdvisorCollector) Collect() io.Reader {
	url := fmt.Sprintf("http://%s:%d/api/v1.2/docker/%s", c.ipAddress, c.port, c.containerName)
	resp, _ := http.Get(url)
	return resp.Body
}

func New(containerName string, ipAddress string, config common.CAdvisorConfig) *cAdvisorCollector {
	return &cAdvisorCollector{
		containerName: containerName,
		ipAddress:     ipAddress,
		port:          config.Port,
	}
}
