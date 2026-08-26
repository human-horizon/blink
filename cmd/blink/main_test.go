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
