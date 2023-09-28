//go:build mage

package main

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/samber/lo"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/constants"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/packages"
)

// Package namespace for handling deb and rpm packages
type Package mg.Namespace

// Deb builds deb package
func (p Package) Deb(arch, packageArch string) error {
	binary := fmt.Sprintf("out/binaries/%s-linux-%s", constants.AppName, arch)
	return p.runPackageScript("deb", arch, packageArch, binary)
}

// Rpm builds rpm package
func (p Package) Rpm(arch, packageArch string) error {
	binary := fmt.Sprintf("out/binaries/%s-linux-%s", constants.AppName, arch)
	return p.runPackageScript("rpm", arch, packageArch, binary)
}

// RpmFips builds rpm package for fips
func (p Package) RpmFips() error {
	arch := "amd64"
	binary := fmt.Sprintf("out/binaries/%s-linux-%s-fips", constants.AppName, arch)
	return p.runPackageScript("rpm-fips", arch, arch, binary)
}

// We use the go generate statement to call the package:generate target through mage
// We don't need any kind of ast parsing as all the data is already available
// This is just an easier way to generate files through mage as we can just run
// go generate for the mage tags
//
//go:generate mage package:generate
var packageBuilds = packages.Builds{
	"deb": {
		{"Deb64", []string{"amd64"}, []string{"amd64"}, []string{"amd64"}},
		{"Deb32", []string{"386"}, []string{"i386"}, []string{"i386"}},
		{"DebArm64", []string{"arm64", "arm64"}, []string{"aarch64", "arm64"}, []string{"aarch64", "arm64"}},
		{"DebArm32", []string{"arm", "arm"}, []string{"armel", "armhf"}, []string{"armel", "armhf"}},
		{"DebRiscv64", []string{"riscv64"}, []string{"riscv64"}, []string{"riscv64"}},
		{"DebIbm", []string{"s390x", "ppc64le"}, []string{"s390x", "ppc64el"}, []string{"s390x", "ppc64le"}},
	},
	"rpm": {
		{"Rpm64", []string{"amd64"}, []string{"amd64"}, []string{"x86_64"}},
		{"Rpm32", []string{"386"}, []string{"i686"}, []string{"i686"}},
		{"RpmArm64", []string{"arm64", "arm64"}, []string{"aarch64", "arm64"}, []string{"aarch64", "arm64"}},
		{"RpmArm32", []string{"arm", "arm"}, []string{"arm", "armhf"}, []string{"arm", "armhf"}},
		{"RpmRiscv64", []string{"riscv64"}, []string{"riscv64"}, []string{"riscv64"}},
		{"RpmIbm", []string{"s390x", "ppc64le"}, []string{"s390x", "ppc64el"}, []string{"s390x", "ppc64le"}},
	},
}

// Archs prints the list of architectures as they appear in the final package's filename
// for either "deb" or "rpm"
func (p Package) Archs(dist string) {
	archs := lo.Flatten(lo.Map(packageBuilds[dist], func(p packages.Build, index int) []string {
		return p.PackageFileArchs
	}))

	fmt.Println(strings.Join(archs, " "))
}

// Filenames prints the final names of the packages for all supported architectures for a version and a distribution
func (p Package) Filenames(dist, version string) error {
	fmt.Println(strings.Join(packages.Filenames(packageBuilds, dist, version), " "))
	return nil
}

type templateContext struct {
	Dist   string
	Builds []packages.Build
}

// Generate generates the Mage package build targets
func (p Package) Generate() error {
	tmpl := `// Code generated by mage package:generate. DO NOT EDIT.
//go:build mage

package main
{{ range .Builds }}
// {{ .Name }} builds {{ $.Dist }} package for {{ .Archs | Join }}
func (p Package) {{ .Name }}() error {
	var err error
	{{ $pkg_archs := .PackageArchs -}}
	{{ range $index, $arch := .Archs -}}
	err = p.{{ $.Dist | Capitalize }}("{{ $arch }}", "{{ index $pkg_archs $index }}")
	if err != nil {
		return err
	}

	{{ end -}}

	return nil
}
{{ end -}}
`

	fns := template.FuncMap{
		"Capitalize": func(in string) string {
			return strings.ToUpper(in[:1]) + in[1:]
		},
		"Join": func(in []string) string {
			return strings.Join(in, " ")
		},
	}

	template, err := template.New("packages").Funcs(fns).Parse(tmpl)
	if err != nil {
		return err
	}

	for dist, b := range packageBuilds {
		f, err := os.OpenFile(fmt.Sprintf("package_%s.go", dist), os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
		if err != nil {
			return err
		}

		defer f.Close()

		if err := template.Execute(f, templateContext{
			Dist:   dist,
			Builds: b,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Deps makes sure the packages needed to build rpm and deb packages are available on the system
func (p Package) Deps() error {
	if err := sh.Run("fpm", "--help"); err != nil {
		return sh.RunV("gem", "install", "rake", "fpm:1.15.1", "--no-document")
	}

	return nil
}

// Prepare prepares the filesystem permissions for packages
func (p Package) Prepare() error {
	err := sh.RunV("bash", "-c", "chmod 755 packaging/root/usr/share/gitlab-runner/")
	if err != nil {
		return err
	}

	err = sh.RunV("bash", "-c", "chmod 755 packaging/root/usr/share/gitlab-runner/*")
	if err != nil {
		return err
	}

	return nil
}

func (Package) runPackageScript(pkgType, arch, packageArch, binary string) error {
	return sh.RunWithV(map[string]string{
		"ARCH":          arch,
		"PACKAGE_ARCH":  packageArch,
		"RUNNER_BINARY": binary,
		"PACKAGE_NAME":  constants.AppName,
		"VERSION":       constants.Version(),
	}, "ci/package", pkgType)
}
