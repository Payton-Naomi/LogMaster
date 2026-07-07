package ai

import (
	"testing"
)

func TestDiagnosisJSON(t *testing.T) {
	d := Diagnosis{
		AnomalyType: "crash",
		Severity:    "ERROR",
		RootCause:   "nil pointer dereference",
		Suggestion:  "check the pointer before use",
	}
	if d.AnomalyType != "crash" {
		t.Fatal("AnomalyType mismatch")
	}
	if d.Severity != "ERROR" {
		t.Fatal("Severity mismatch")
	}
}