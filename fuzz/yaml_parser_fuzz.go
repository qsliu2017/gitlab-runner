package fuzz

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	yml_parser "gitlab.com/gitlab-org/gitlab-runner/helpers/gitlab_ci_yaml_parser"
	"io/ioutil"
	"os"
)

func prepareTestFile(fileContent []byte) string {
	file, _ := ioutil.TempFile("", "gitlab-ci-yml")
	defer file.Close()

	_, _ = file.Write(fileContent)
	return file.Name()
}

func parseYaml(fileContent []byte) {
	file := prepareTestFile(fileContent)
	defer os.Remove(file)

	parser := &yml_parser.GitLabCiYamlParser{}
	parser.SetFilename(file)
	parser.SetJobName("job-name")

	jobResponse := &common.JobResponse{}
	parser.ParseYaml(jobResponse)
}

func Fuzz(data []byte) int {
	parseYaml(data)
	return 0
}
