package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/checker"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/lexer"
	"github.com/humanhorizon/blink/internal/parser"
)

func main() {
	if path := os.Getenv("BLINK_HEAPPROFILE"); path != "" {
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGUSR1)
			for range sig {
				f, err := os.Create(path)
				if err == nil {
					_ = pprof.WriteHeapProfile(f)
					_ = f.Close()
				}
			}
		}()
	}
	stopMemoryGuard, err := startMemoryGuard()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer stopMemoryGuard()

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: blink <check|build|run> <path>")
		os.Exit(2)
	}
	cmd := os.Args[1]
	path := os.Args[2]
	switch cmd {
	case "check":
		if err := checkPath(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "build":
		_, err := buildPath(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "run":
		binary, err := buildPath(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		out, err := exec.Command(binary).CombinedOutput()
		if err != nil {
			exitErr, ok := err.(*exec.ExitError)
			if ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Stdout.Write(out)
	default:
		fmt.Fprintln(os.Stderr, "usage: blink <check|build|run> <path>")
		os.Exit(2)
	}
}

func buildPath(path string) (string, error) {
	files, paths, modPaths, err := loadModules(path)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no .rs files found in %s", path)
	}
	rep := &diag.Reporter{}
	chk := checker.New(files, paths, rep, modPaths)
	if !chk.Check() {
		return "", fmt.Errorf("%s", rep.String())
	}
	cCode := chk.GenerateC()
	buildDir := filepath.Join(path, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", err
	}
	cPath := filepath.Join(buildDir, "out.c")
	if err := os.WriteFile(cPath, []byte(cCode), 0644); err != nil {
		return "", err
	}
	binary := filepath.Join(buildDir, "out")
	tcc, err := findTCC()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(tcc, "-o", binary, cPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tcc failed: %w\n%s", err, string(out))
	}
	return binary, nil
}

func startMemoryGuard() (func(), error) {
	limit, err := configuredMemoryLimit()
	if err != nil {
		return func() {}, err
	}
	debug.SetMemoryLimit(limit)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				if stats.HeapAlloc >= uint64(limit) {
					fmt.Fprintf(os.Stderr, "blink: memory limit exceeded (%d bytes)\\n", limit)
					os.Exit(1)
				}
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }, nil
}

func configuredMemoryLimit() (int64, error) {
	value := os.Getenv("BLINK_MEMLIMIT")
	if value == "" {
		value = "1GiB"
	}
	return parseMemoryLimit(value)
}

func parseMemoryLimit(value string) (int64, error) {
	value = strings.TrimSpace(value)
	multipliers := map[string]uint64{
		"MiB": 1 << 20,
		"GiB": 1 << 30,
	}
	for suffix, multiplier := range multipliers {
		if !strings.HasSuffix(value, suffix) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(value, suffix))
		parsed, err := strconv.ParseUint(number, 10, 64)
		if err != nil || parsed == 0 || parsed > uint64((1<<63-1)/int64(multiplier)) {
			break
		}
		return int64(parsed * multiplier), nil
	}
	return 0, fmt.Errorf("invalid BLINK_MEMLIMIT %q; use a positive integer with MiB or GiB", value)
}

func findTCC() (string, error) {
	if configured := os.Getenv("BLINK_TCC"); configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
	}
	if tcc, err := exec.LookPath("tcc"); err == nil {
		return tcc, nil
	}
	if executable, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(executable), "tcc")
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}
	return "", fmt.Errorf("tcc not found; set BLINK_TCC or add tcc to PATH")
}

func checkPath(path string) error {
	files, paths, modPaths, err := loadModules(path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .rs files found in %s", path)
	}
	if os.Getenv("BLINK_DEBUG") != "" {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(os.Stderr, "loaded %d files, heap_alloc=%d bytes\n", len(files), ms.HeapAlloc)
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
	rootDir := dir
	root, err := findRootFile(rootDir, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if root == "" {
		rootDir = filepath.Join(dir, "src")
		root, err = findRootFile(rootDir, false)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if root == "" {
		return nil, nil, nil, fmt.Errorf("no root .rs file in %s", dir)
	}
	return loadFile(filepath.Join(rootDir, root), []string{}, rootDir)
}

func findRootFile(dir string, allowAnyRustFile bool) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
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
		if allowAnyRustFile && root == "" && strings.HasSuffix(name, ".rs") {
			root = name
		}
	}
	return root, nil
}

func loadFile(path string, modPath []string, dir string) ([]*ast.File, []string, [][]string, error) {
	if os.Getenv("BLINK_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "loading %s\n", path)
	}
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
	if os.Getenv("BLINK_DEBUG") != "" {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(os.Stderr, "  parsed %s: %d decls, heap_alloc=%d bytes\n", path, len(file.Decls), ms.HeapAlloc)
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
