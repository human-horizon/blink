package main

import (
	"testing"
)

func TestCheckValid(t *testing.T) {
	err := checkPath("../../testdata/phase1/valid")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckInvalid(t *testing.T) {
	err := checkPath("../../testdata/phase1/invalid")
	if err == nil {
		t.Fatal("expected error for invalid testdata")
	}
}
