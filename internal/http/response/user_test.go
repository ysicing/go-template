package response

import (
	"testing"

	"github.com/ysicing/go-template/model"
)

func TestNewUserResponseIncludesPermissionList(t *testing.T) {
	user := &model.User{
		Base: model.Base{
			ID: "user-id",
		},
		Username:      "alice",
		Email:         "alice@example.com",
		IsAdmin:       true,
		Provider:      "local",
		EmailVerified: true,
	}
	user.SetPermissions([]string{model.PermissionAdminStatsRead})

	resp := NewUserResponse(user)
	if resp.ID != user.ID || resp.Username != user.Username || !resp.IsAdmin {
		t.Fatalf("unexpected user response: %#v", resp)
	}
	if len(resp.Permissions) != 1 || resp.Permissions[0] != model.PermissionAdminStatsRead {
		t.Fatalf("expected permission list, got %#v", resp.Permissions)
	}
}

func TestNewUserResponseNilUser(t *testing.T) {
	resp := NewUserResponse(nil)
	if resp.ID != "" || len(resp.Permissions) != 0 {
		t.Fatalf("expected zero response, got %#v", resp)
	}
}
