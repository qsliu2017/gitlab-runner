package utils

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
)

func MachineNameTemplate(config *common.RunnerConfig) string {
	runner := strings.ToLower(dns.MakeRFC1123Compatible(config.ShortDescription()))
	if runner == "" {
		return config.Machine.MachineName
	}

	return "runner-" + strings.ToLower(dns.MakeRFC1123Compatible(config.ShortDescription())) +
		"-" + config.Machine.MachineName
}

func FilterMachineListByNameTemplate(machineNames []string, machineNameTemplate string) []string {
	newMachineNames := make([]string, 0, len(machineNames))
	for _, name := range machineNames {
		if MatchesMachineNameTemplate(name, machineNameTemplate) {
			newMachineNames = append(newMachineNames, name)
		}
	}

	return newMachineNames
}

func MatchesMachineNameTemplate(name string, machineNameTemplate string) bool {
	var query string
	n, _ := fmt.Sscanf(name, machineNameTemplate, &query)

	return n == 1
}

func NewMachineName(config *common.RunnerConfig) string {
	random := make([]byte, 4)
	_, _ = rand.Read(random)

	currentTime := time.Now().Unix()

	return fmt.Sprintf(MachineNameTemplate(config), fmt.Sprintf("%d-%x", currentTime, random))
}
