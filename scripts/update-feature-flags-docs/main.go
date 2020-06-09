package main

import (
	"bytes"
	"fmt"
	"text/template"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/scripts/internal/block-line-replacer"
)

const (
	docsFile = "./docs/configuration/feature-flags.md"

	startPlaceholder = "<!-- feature_flags_list_start -->"
	endPlaceholder   = "<!-- feature_flags_list_end -->"
)

var ffTableTemplate = `{{ placeholder "start" }}

| Feature flag | Default value | Deprecated | To be removed with | Description |
|--------------|---------------|------------|--------------------|-------------|
{{ range $_, $flag := . -}}
| {{ $flag.Name | raw }} | {{ $flag.DefaultValue | raw }} | {{ $flag.Deprecated | tick }} | {{ $flag.ToBeRemovedWith }} | {{ $flag.Description }} |
{{ end }}
{{ placeholder "end" }}
`

func main() {
	replacer := blocklinereplacer.New(startPlaceholder, endPlaceholder)
	fileReplacer := blocklinereplacer.NewFileReplacer(docsFile, replacer)

	err := fileReplacer.Replace(prepareTable())
	if err != nil {
		panic(fmt.Sprintf("Error while replacing file content: %v", err))
	}
}

func prepareTable() string {
	tpl := template.New("ffTable")
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
		"raw": func(input string) string {
			return fmt.Sprintf("`%s`", input)
		},
		"tick": func(input bool) string {
			if input {
				return "✓"
			}

			return "✗"
		},
	})

	tpl, err := tpl.Parse(ffTableTemplate)
	if err != nil {
		panic(fmt.Sprintf("Error while parsing the template: %v", err))
	}

	buffer := new(bytes.Buffer)

	err = tpl.Execute(buffer, featureflags.GetAll())
	if err != nil {
		panic(fmt.Sprintf("Error while executing the template: %v", err))
	}

	return buffer.String()
}
