package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/checker"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/lexer"
	"github.com/humanhorizon/blink/internal/parser"
)

func main() {
	if len(os.Args) < 3 || os.Args[1] != "check" {
		fmt.Fprintln(os.Stderr, "usage: blink check <path>")
		os.Exit(2)
	}
	path := os.Args[2]
	if err := checkPath(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func checkPath(path string) error {
	files, paths, modPaths, err := loadModules(path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .rs files found in %s", path)
	}
	rep := &diag.Reporter{}
	chk := checker.New(files, paths, rep, modPaths)
	chk.Check()
	if rep.HasErrors() {
		return fmt.Errorf("%s", rep.String())
	}
	return nil
}

func loadModules(dir string) ([]*ast.File, []string, [][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	var root string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "main.rs" || name == "lib.rs" {
			root = name
			break
		}
		if root == "" && strings.HasSuffix(name, ".rs") {
			root = name
		}
	}
	if root == "" {
		return nil, nil, nil, fmt.Errorf("no root .rs file in %s", dir)
	}
	return loadFile(filepath.Join(dir, root), []string{}, dir)
}

func loadFile(path string, modPath []string, dir string) ([]*ast.File, []string, [][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	l := lexer.New(data)
	p := parser.New(l, data)
	file, err := p.ParseFile()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse error in %s: %w", path, err)
	}
	var childFiles []*ast.File
	var childPaths []string
	var childModPaths [][]string
	for _, d := range file.Decls {
		mod, ok := d.(*ast.ModDecl)
		if !ok || mod.Inline != nil {
			continue
		}
		childPath := filepath.Join(dir, mod.File)
		childModPath := append(append([]string{}, modPath...), mod.Name)
		cfs, cps, cmps, err := loadFile(childPath, childModPath, dir)
		if err != nil {
			return nil, nil, nil, err
		}
		childFiles = append(childFiles, cfs...)
		childPaths = append(childPaths, cps...)
		childModPaths = append(childModPaths, cmps...)
	}
	resultFiles := append([]*ast.File{file}, childFiles...)
	resultPaths := append([]string{path}, childPaths...)
	resultModPaths := append([][]string{modPath}, childModPaths...)
	return resultFiles, resultPaths, resultModPaths, nil
}
