package main

import (
	"bytes"
	"fmt"
	"text/template"

	"gitlab.com/gitlab-org/gitlab-runner/scripts/internal/block-line-replacer"
)

const (
	docsFile = "./docs/install/linux-repository.md"

	startPlaceholder = "<!-- distribution_versions_table_start -->"
	endPlaceholder   = "<!-- distribution_versions_table_end -->"
)

var versionsTableTemplate = `{{ placeholder "start" }}

| Distribution | Version | End of Life date |
|--------------|---------|------------------|
{{ range $_, $distribution := . -}}
{{ range $_, $versionInfo := $distribution.Versions -}}
| {{ $distribution.Name }} | {{ $versionInfo.Version }} | {{ $versionInfo.EOL }} |
{{ end -}}
{{ end }}
{{ placeholder "end" }}
`

func updateDocs() {
	replacer := blocklinereplacer.New(startPlaceholder, endPlaceholder)
	fileReplacer := blocklinereplacer.NewFileReplacer(docsFile, replacer)

	err := fileReplacer.Replace(prepareTable())
	if err != nil {
		panic(fmt.Sprintf("Error while replacing file content: %v", err))
	}
}

func prepareTable() string {
	tpl := template.New("versionsTable")
	tpl.Funcs(template.FuncMap{
		"placeholder": func(placeholderType string) string {
			switch placeholderType {
			case "start":
				return startPlaceholder
			case "end":
				return endPlaceholder
			default:
				panic(fmt.Sprintf("Undefined placeholder type %q", placeholderType))
			}
		},
	})

	tpl, err := tpl.Parse(versionsTableTemplate)
	if err != nil {
		panic(fmt.Sprintf("Error while parsing the template: %v", err))
	}

	buffer := new(bytes.Buffer)

	err = tpl.Execute(buffer, distributions.Distributions)
	if err != nil {
		panic(fmt.Sprintf("Error while executing the template: %v", err))
	}

	return buffer.String()
}
