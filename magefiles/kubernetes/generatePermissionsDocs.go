package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

const (
	startPlaceholder = "<!-- k8s_api_permissions_list_start -->"
	endPlaceholder   = "<!-- k8s_api_permissions_list_end -->"
	docsFilePath     = "docs/executors/kubernetes.md"
)

var tableTemplate = fmt.Sprintf(` %s

| Resource | Verb | Feature flag |
|----------|------|--------------|
{{ range $_, $permission := . -}}
| {{ $permission.resource }} | {{ $permission.verb }} | {{ $permission.ff }}={{ $permission.ffValue }} |
{{ end }}

%s`, startPlaceholder, endPlaceholder)

func GeneratePermissionsDocs() error {
	permissions, err := parsePermissions()
	if err != nil {
		return err
	}

	docsFile, err := os.ReadFile(docsFilePath)
	if err != nil {
		return err
	}

	table, err := renderTable(permissions)
	if err != nil {
		return err
	}

	newDocsFile, err := replace(string(docsFile), table)
	if err != nil {
		return err
	}

	if err := os.WriteFile(docsFilePath, []byte(newDocsFile), 0o644); err != nil {
		return fmt.Errorf("error while writing new content for %q file: %w", docsFile, err)
	}

	return nil
}

func renderTable(permissions []permission) (string, error) {
	tpl := template.New("permissionsTable")
	tpl, err := tpl.Parse(tableTemplate)
	if err != nil {
		return "", err
	}

	buffer := new(bytes.Buffer)

	err = tpl.Execute(buffer, permissions)
	if err != nil {
		panic(fmt.Sprintf("Error while executing the template: %v", err))
	}

	return buffer.String(), nil
}

func replace(fileContent, tableContent string) (string, error) {
	replacer := newBlockLineReplacer(startPlaceholder, endPlaceholder, fileContent, tableContent)

	newContent, err := replacer.Replace()
	if err != nil {
		return "", fmt.Errorf("error while replacing the content: %w", err)
	}

	return newContent, nil
}
