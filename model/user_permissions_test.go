package model

import (
	"slices"
	"testing"
)

func TestUserPermissionList_FromAdminWithoutExplicitPermissions(t *testing.T) {
	u := User{IsAdmin: true}
	perms := u.PermissionList()

	if len(perms) != len(allAdminPermissions) {
		t.Fatalf("expected %d permissions, got %d", len(allAdminPermissions), len(perms))
	}
	for _, required := range allAdminPermissions {
		if !slices.Contains(perms, required) {
			t.Fatalf("missing permission %q", required)
		}
	}
}

func TestUserSetPermissions_NormalizesAndSorts(t *testing.T) {
	u := &User{}
	u.SetPermissions([]string{" admin.settings.read ", "admin.users.read", "admin.users.read", ""})

	if u.Permissions != "admin.settings.read,admin.users.read" {
		t.Fatalf("unexpected permissions string: %q", u.Permissions)
	}
}

func TestUserHasPermission_WithExplicitPermissions(t *testing.T) {
	u := User{Permissions: "admin.users.read,admin.settings.write"}

	if !u.HasPermission(PermissionAdminUsersRead) {
		t.Fatal("expected users read permission")
	}
	if u.HasPermission(PermissionAdminPointsWrite) {
		t.Fatal("did not expect points write permission")
	}
}

func TestIsValidPermission(t *testing.T) {
	if !IsValidPermission(PermissionAdminStatsRead) {
		t.Fatal("expected admin.stats.read to be valid")
	}
	if IsValidPermission("admin.invalid.permission") {
		t.Fatal("expected unknown permission to be invalid")
	}
}
