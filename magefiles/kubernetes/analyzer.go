package kubernetes

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

type simplePosition struct {
	fileName string
	line     int
}

type permission struct {
	resource string
	verb     string
	ff       string
	ffValue  string
}

var commentRegex = regexp.MustCompile(`//\s?kubeAPI:\s?(\w+),\s?(\w+)(,?\s?((FF_.+)=(.+)))?`)

func parsePermissions() ([]permission, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseDir(fset, "executors/kubernetes", nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	positions := map[simplePosition]token.Pos{}

	var permissions []permission

	for _, pkg := range f {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(node ast.Node) bool {
				expr, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := expr.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				if sel.Sel.Name != "CoreV1" {
					return true
				}

				selectorIdents, ok := sel.X.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				// TODO: Check for the type of the field instead of the name
				if selectorIdents.Sel.Name != "kubeClient" && selectorIdents.Sel.Name != "KubeClient" {
					return true
				}

				callPosition := fset.Position(node.Pos())
				sp := simplePosition{
					fileName: callPosition.Filename,
					line:     callPosition.Line - 1,
				}
				positions[sp] = node.Pos()

				return true
			})

			for _, commentGroup := range f.Comments {
				for _, comment := range commentGroup.List {
					position := fset.Position(comment.Pos())
					sp := simplePosition{
						fileName: position.Filename,
						line:     position.Line,
					}
					if _, ok := positions[sp]; ok {
						matches := commentRegex.FindAllStringSubmatch(comment.Text, -1)
						if len(matches) == 0 {
							continue
						}

						m := matches[0]
						resource := m[1]
						verb := m[2]
						ff := m[5]
						ffValue := m[6]

						permissions = append(permissions, permission{
							resource: resource,
							verb:     verb,
							ff:       ff,
							ffValue:  ffValue,
						})

						// TODO: make these checks more robust based on the called methods instead of the comment
						delete(positions, sp)
					}
				}
			}
		}

	}

	var errs []string
	for _, pos := range positions {
		errs = append(errs, fmt.Sprintf("%v Missing Kube API annotations.", fset.Position(pos)))
	}

	if len(errs) == 0 {
		return permissions, nil
	}

	return nil, errors.New(fmt.Sprintf("%s\n\nAnnotations must be written as comments directly above each `kubeClient.CoreV1` call and in the format of // kubeAPI: <resource>, <verb>, <FF=VALUE>(optional)", strings.Join(errs, "\n")))
}
