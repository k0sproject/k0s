/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"regexp"
	"strings"
)

var (
	// next line starts with a capital letter, quote or + or - sign
	startsNewLine = regexp.MustCompile(`^(?://|/*)\s{0,}[A-Z"'+-]`)

	endOfSentence = regexp.MustCompile(`[.?!:]\s{0,}$`)

	// directive comment keywords
	directiveComment = regexp.MustCompile(`//\s{0,}\+(kubebuilder|optional|groupName|go:|nolint:)`)

	// there are some comments that contain a bulleted list of words
	wordlistComment = regexp.MustCompile(`^\s{0,}//\s{0,}[-*][A-Za-z0-9-.@]$`)

	// comments that end with a URL should not be changed to avoid breaking the URL
	endsWithURL = regexp.MustCompile(`https?://\S*$`)

	// dont modify empty comments
	isEmptyComment = regexp.MustCompile(`^\s{0,}//\s{0,}$`)
)

// ProcessFile processes a file's comments and returns the result as a reader.
func ProcessFile(filename string, reader io.Reader) (io.Reader, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, reader, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var changed bool
	for _, decl := range node.Decls {
		switch x := decl.(type) {
		case *ast.FuncDecl:
			// For functions
			if x.Doc != nil && isExported(x.Name.Name) {
				changed = processComments(x.Doc)
			}
		case *ast.GenDecl:
			// struct types, variables, and constants
			for _, spec := range x.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if structType, ok := s.Type.(*ast.StructType); ok && isExported(s.Name.Name) {
						changed = processStructType(structType)
					} else if isExported(s.Name.Name) {
						changed = processComments(s.Doc)
					}
				case *ast.ValueSpec:
					// variable and constant declarations
					changed = processValueSpecs(s)
				}
			}
		}
	}

	if !changed {
		return nil, fmt.Errorf("no changes made to %s", filename)
	}

	out := bytes.NewBuffer(nil)

	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(out, fset, node); err != nil {
		return nil, err
	}
	return out, nil
}

func isExported(name string) bool {
	return strings.HasPrefix(name, strings.ToUpper(string(name[0])))
}

func processComments(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	var changed bool

	for idx, comment := range cg.List {
		if directiveComment.MatchString(comment.Text) {
			continue
		}

		if wordlistComment.MatchString(comment.Text) {
			continue
		}

		if endsWithURL.MatchString(comment.Text) && !strings.HasSuffix(comment.Text, ")") && !strings.HasSuffix(comment.Text, "]") {
			continue
		}

		if isEmptyComment.MatchString(comment.Text) {
			continue
		}

		if !endOfSentence.MatchString(comment.Text) {
			if idx+1 < len(cg.List) {
				nextRow := cg.List[idx+1]
				if startsNewLine.MatchString(nextRow.Text) || isEmptyComment.MatchString(nextRow.Text) {
					cg.List[idx].Text += "."
					changed = true
				}
			} else {
				cg.List[idx].Text += "."
				changed = true
			}
		}
	}

	return changed
}

func processStructType(structType *ast.StructType) bool {
	if structType.Fields == nil {
		return false
	}
	var changed bool
	for _, field := range structType.Fields.List {
		if field.Doc != nil {
			if processComments(field.Doc) {
				changed = true
			}
		}
	}
	return changed
}

func processValueSpecs(valueSpec *ast.ValueSpec) bool {
	if valueSpec.Doc != nil {
		for _, name := range valueSpec.Names {
			if isExported(name.Name) {
				return processComments(valueSpec.Doc)
				break
			}
		}
	}
	return false
}
