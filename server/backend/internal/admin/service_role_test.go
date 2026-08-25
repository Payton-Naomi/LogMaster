package admin

import (
	"context"
	"testing"
)

func TestRoleForUserRequiresConfiguredResolver(t *testing.T) {
	service := &Service{}
	if _, err := service.roleForUser(context.Background(), "ou_test"); err == nil {
		t.Fatal("expected an error when the read-only role resolver is not configured")
	}
}
