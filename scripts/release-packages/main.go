package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type List struct {
	Distributions []Distribution `yaml:"distributions"`
}

type Distribution struct {
	Name     string        `yaml:"name"`
	Type     PackageType   `yaml:"type"`
	Versions []VersionInfo `yaml:"versions"`
}

type PackageType string

type VersionInfo struct {
	Version string `yaml:"version"`
	EOL     string `yaml:"eol,omitempty"`

	IDs               []string `yaml:"ids"`
	PublishToUnstable bool     `yaml:"publish_to_unstable,omitempty"`
}

const (
	PackageDEB PackageType = "deb"
	PackageRPM PackageType = "rpm"

	configFile = "./.gitlab/package_versions.yml"
)

var (
	distributions List

	packageName          string
	packageCloudURL      string
	packageCloudRepo     string
	packageCloudRepoBeta string
)

func init() {
	readConfig()

	packageName = os.Getenv("PACKAGE_NAME")
	packageCloudURL = os.Getenv("PACKAGE_CLOUD_URL")
	packageCloudRepo = os.Getenv("PACKAGE_CLOUD")
	packageCloudRepoBeta = os.Getenv("PACKAGE_CLOUD_BETA")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		list()
	case "update-docs":
		updateDocs()
	case "release":
		release()
	case "yank":
		yank()
	default:
		printUsage()
		os.Exit(1)
	}
}

func readConfig() {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(fmt.Sprintf("Error while reading configuration file %q: %v", configFile, err))
	}

	err = yaml.Unmarshal(data, &distributions)
	if err != nil {
		panic(fmt.Sprintf("Error while parsing configuration file %q: %v", configFile, err))
	}
}

func printUsage() {
	fmt.Printf(`Usage:
%s [command]

Commands:
  list                      List supported distributions
  update-docs               Updates supported version table in the documentation
  release [stable|bleeding] Releases packages for defined distributions
  yank [version]            Removes defined package version for defined distributions

`, os.Args[0])
}
