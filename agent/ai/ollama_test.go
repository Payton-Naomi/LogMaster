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
		t.Fatal("AnomalyType 不匹配")
	}
	if d.Severity != "ERROR" {
		t.Fatal("Severity 不匹配")
	}
}