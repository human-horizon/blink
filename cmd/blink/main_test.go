package main

import (
	"testing"
)

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
