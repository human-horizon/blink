package main

import (
	"fmt"
	"os"

	"github.com/humanhorizon/blink/internal/checker"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/lexer"
	"github.com/humanhorizon/blink/internal/parser"
	"github.com/humanhorizon/blink/internal/source"
	"github.com/humanhorizon/blink/internal/ast"
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
	fs, err := source.LoadDir(path)
	if err != nil {
		return err
	}
	if len(fs.Files) == 0 {
		return fmt.Errorf("no .rs files found in %s", path)
	}
	var files []*ast.File
	var paths []string
	rep := &diag.Reporter{}
	for _, f := range fs.Files {
		l := lexer.New(f.Content)
		p := parser.New(l, f.Content)
		file, err := p.ParseFile()
		if err != nil {
			rep.Errorf(f.Path, 1, 1, "parse error: %v", err)
			continue
		}
		files = append(files, file)
		paths = append(paths, f.Path)
	}
	chk := checker.New(files, paths, rep)
	chk.Check()
	if rep.HasErrors() {
		return fmt.Errorf("%s", rep.String())
	}
	return nil
}
