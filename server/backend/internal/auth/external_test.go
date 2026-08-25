package auth

import "testing"

func TestExternalPasswordHash(t *testing.T) {
	hash, err := hashPassword("a secure contractor password")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "a secure contractor password") {
		t.Fatal("expected password verification to succeed")
	}
	if verifyPassword(hash, "wrong password") {
		t.Fatal("expected wrong password verification to fail")
	}
}

func TestValidateExternalRegistration(t *testing.T) {
	user, err := validateExternalRegistration(externalRegisterRequest{
		Name: "Contractor", Email: "CONTRACTOR@example.com", Company: "Example Supplier",
		Password: "a secure contractor password", ConfirmPassword: "a secure contractor password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.IdentityType != "external" || user.Role != "user" || user.Email != "contractor@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if _, err := validateExternalRegistration(externalRegisterRequest{
		Name: "Contractor", Email: "contractor@example.com", Company: "Example Supplier",
		Password: "", ConfirmPassword: "",
	}); err == nil {
		t.Fatal("expected empty password to be rejected")
	}
}

func TestExternalRegistrationAllowsShortPassword(t *testing.T) {
	if _, err := validateExternalRegistration(externalRegisterRequest{
		Name: "Contractor", Email: "contractor@example.com", Company: "Example Supplier",
		Password: "123", ConfirmPassword: "123",
	}); err != nil {
		t.Fatalf("short non-empty password should be allowed: %v", err)
	}
}

func TestExternalRegistrationRequiresMatchingPassword(t *testing.T) {
	if _, err := validateExternalRegistration(externalRegisterRequest{
		Name: "Contractor", Email: "contractor@example.com", Company: "Example Supplier",
		Password: "a secure contractor password", ConfirmPassword: "different password",
	}); err == nil {
		t.Fatal("expected mismatched passwords to be rejected")
	}
}
