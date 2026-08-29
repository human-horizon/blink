package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		value string
		want  int64
	}{
		{value: "512MiB", want: 512 << 20},
		{value: "1GiB", want: 1 << 30},
	}
	for _, test := range tests {
		got, err := parseMemoryLimit(test.value)
		if err != nil || got != test.want {
			t.Fatalf("parseMemoryLimit(%q) = %d, %v; want %d", test.value, got, err, test.want)
		}
	}
	for _, value := range []string{"", "0GiB", "1GB", "GiB"} {
		if _, err := parseMemoryLimit(value); err == nil {
			t.Fatalf("parseMemoryLimit(%q) unexpectedly succeeded", value)
		}
	}
}

func TestLoadModulesCargoLayout(t *testing.T) {
	tests := []struct {
		name string
		root string
	}{
		{name: "library", root: "lib.rs"},
		{name: "binary", root: "main.rs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			srcDir := filepath.Join(dir, "src")
			if err := os.Mkdir(srcDir, 0755); err != nil {
				t.Fatal(err)
			}
			root := "mod helper;\n"
			if test.root == "main.rs" {
				root += "fn main() {}\n"
			}
			if err := os.WriteFile(filepath.Join(srcDir, test.root), []byte(root), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(srcDir, "helper.rs"), []byte("fn helper() {}\n"), 0644); err != nil {
				t.Fatal(err)
			}

			files, paths, _, err := loadModules(dir)
			if err != nil {
				t.Fatalf("loadModules failed: %v", err)
			}
			if len(files) != 2 || len(paths) != 2 {
				t.Fatalf("expected root and module files, got %d files and %d paths", len(files), len(paths))
			}
			if paths[0] != filepath.Join(srcDir, test.root) {
				t.Fatalf("expected root path %q, got %q", filepath.Join(srcDir, test.root), paths[0])
			}
		})
	}
}

func TestCheckValidPhase1(t *testing.T) {
	err := checkPath("../../testdata/phase1/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase1(t *testing.T) {
	err := checkPath("../../testdata/phase1/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase2(t *testing.T) {
	err := checkPath("../../testdata/phase2/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase2(t *testing.T) {
	err := checkPath("../../testdata/phase2/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase3(t *testing.T) {
	err := checkPath("../../testdata/phase3/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase3(t *testing.T) {
	err := checkPath("../../testdata/phase3/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase4(t *testing.T) {
	err := checkPath("../../testdata/phase4/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase4(t *testing.T) {
	err := checkPath("../../testdata/phase4/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase5(t *testing.T) {
	err := checkPath("../../testdata/phase5/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase5(t *testing.T) {
	err := checkPath("../../testdata/phase5/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase6(t *testing.T) {
	err := checkPath("../../testdata/phase6/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase6(t *testing.T) {
	err := checkPath("../../testdata/phase6/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase7(t *testing.T) {
	err := checkPath("../../testdata/phase7/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase7(t *testing.T) {
	err := checkPath("../../testdata/phase7/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase8(t *testing.T) {
	err := checkPath("../../testdata/phase8/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase8(t *testing.T) {
	err := checkPath("../../testdata/phase8/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase9(t *testing.T) {
	err := checkPath("../../testdata/phase9/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase9(t *testing.T) {
	err := checkPath("../../testdata/phase9/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase10(t *testing.T) {
	err := checkPath("../../testdata/phase10/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase10(t *testing.T) {
	err := checkPath("../../testdata/phase10/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestCheckValidPhase11(t *testing.T) {
	err := checkPath("../../testdata/phase11/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalidPhase11(t *testing.T) {
	err := checkPath("../../testdata/phase11/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}

func TestBuildRunExit42(t *testing.T) {
	binary, err := buildPath("../../testdata/run/valid/exit42")
	if err != nil {
		t.Fatalf("expected build success, got: %v", err)
	}
	cmd := exec.Command(binary)
	if err := cmd.Run(); err == nil {
		t.Fatal("expected exit code 42")
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 42 {
		t.Fatalf("expected exit code 42, got: %v", err)
	}
}

func TestBuildValidPhases(t *testing.T) {
	phases := []string{
		"phase1", "phase2", "phase3", "phase4", "phase5", "phase6",
		"phase7", "phase8", "phase9", "phase10", "phase11",
	}
	for _, phase := range phases {
		if _, err := buildPath("../../testdata/" + phase + "/valid"); err != nil {
			t.Fatalf("expected %s to build, got: %v", phase, err)
		}
	}
}
