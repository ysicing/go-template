package user

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestServiceListUsersByKeywordRoleStatus(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "active")
	createTestUser(t, conn, "bob", "bob@example.com", RoleAdmin, "disabled")

	result, err := service.ListUsers(ListUsersQuery{
		Keyword:  "bob",
		Role:     string(RoleAdmin),
		Status:   "disabled",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Username != "bob" {
		t.Fatalf("expected bob, got %s", result.Items[0].Username)
	}
}

func TestServiceGetUser(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "active")

	found, err := service.GetUser(account.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if found.ID != account.ID {
		t.Fatalf("expected user id %d, got %d", account.ID, found.ID)
	}
}

func TestServiceDisableUserRejectsSelf(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	admin := createTestUser(t, conn, "admin", "admin@example.com", RoleAdmin, "active")

	err := service.DisableUser(admin.ID, admin.ID)
	if !errors.Is(err, ErrCannotDisableSelf) {
		t.Fatalf("expected ErrCannotDisableSelf, got %v", err)
	}
}

func TestServiceEnableUser(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "disabled")

	if err := service.EnableUser(account.ID); err != nil {
		t.Fatalf("enable user: %v", err)
	}

	var updated User
	if err := conn.First(&updated, account.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.Status != "active" {
		t.Fatalf("expected active, got %s", updated.Status)
	}
}

func TestServiceChangePassword(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUserWithPassword(t, conn, "user1", "user1@example.com", "oldpass123")

	err := service.ChangePassword(account.ID, ChangePasswordInput{
		OldPassword:        "oldpass123",
		NewPassword:        "newpass123",
		ConfirmNewPassword: "newpass123",
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}

	var updated User
	if err := conn.First(&updated, account.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("newpass123")); err != nil {
		t.Fatalf("expected password updated: %v", err)
	}
}

func TestServiceCreateUser(t *testing.T) {
	service, conn := newUserServiceForTest(t)

	account, err := service.CreateUser(CreateUserInput{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "password123",
		Role:     string(RoleAdmin),
		Status:   "active",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if account.ID == 0 {
		t.Fatal("expected persisted user id")
	}
	if account.PasswordHash == "" || account.PasswordHash == "password123" {
		t.Fatal("expected hashed password")
	}

	var stored User
	if err := conn.First(&stored, account.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Username != "charlie" {
		t.Fatalf("expected charlie, got %s", stored.Username)
	}
}

func TestServiceCreateUserRejectsDuplicates(t *testing.T) {
	cases := []struct {
		name  string
		input CreateUserInput
		want  error
	}{
		{name: "username", input: CreateUserInput{Username: "charlie", Email: "charlie2@example.com", Password: "password123", Role: string(RoleUser), Status: "active"}, want: ErrDuplicateUsername},
		{name: "email", input: CreateUserInput{Username: "charlie2", Email: "charlie@example.com", Password: "password123", Role: string(RoleUser), Status: "active"}, want: ErrDuplicateEmail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, conn := newUserServiceForTest(t)
			createTestUser(t, conn, "charlie", "charlie@example.com", RoleUser, "active")
			_, err := service.CreateUser(tc.input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestServiceCreateUserRejectsInvalidRole(t *testing.T) {
	service, _ := newUserServiceForTest(t)

	_, err := service.CreateUser(CreateUserInput{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "password123",
		Role:     "super-admin",
		Status:   "active",
	})
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestServiceUpdateUser(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "active")

	updated, err := service.UpdateUser(account.ID, UpdateUserInput{
		Username: "alice2",
		Email:    "alice2@example.com",
		Role:     string(RoleAdmin),
		Status:   "disabled",
	})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Username != "alice2" {
		t.Fatalf("expected alice2, got %s", updated.Username)
	}
	if updated.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %s", updated.Role)
	}
	if updated.Status != "disabled" {
		t.Fatalf("expected disabled, got %s", updated.Status)
	}

	var persisted User
	if err := conn.First(&persisted, account.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if persisted.Username != "alice2" || persisted.Email != "alice2@example.com" {
		t.Fatalf("expected persisted profile updated, got %s/%s", persisted.Username, persisted.Email)
	}
	if persisted.Role != RoleAdmin || persisted.Status != "disabled" {
		t.Fatalf("expected persisted admin/disabled, got %s/%s", persisted.Role, persisted.Status)
	}
}

func TestServiceUpdateUserRejectsInvalidStatus(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "active")

	_, err := service.UpdateUser(account.ID, UpdateUserInput{
		Username: "alice",
		Email:    "alice@example.com",
		Role:     string(RoleUser),
		Status:   "pending",
	})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestServiceDeleteUserSoftDeletes(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	admin := createTestUser(t, conn, "admin", "admin@example.com", RoleAdmin, "active")
	target := createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "active")

	if err := service.DeleteUser(admin.ID, target.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if err := conn.First(&User{}, target.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found, got %v", err)
	}

	var deleted User
	if err := conn.Unscoped().First(&deleted, target.ID).Error; err != nil {
		t.Fatalf("load deleted user: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatal("expected deleted at to be set")
	}
}

func TestServiceDeleteUserRejectsSelf(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	admin := createTestUser(t, conn, "admin", "admin@example.com", RoleAdmin, "active")

	err := service.DeleteUser(admin.ID, admin.ID)
	if !errors.Is(err, ErrCannotDeleteSelf) {
		t.Fatalf("expected ErrCannotDeleteSelf, got %v", err)
	}
}

func TestServiceChangePasswordRejectsWrongOldPassword(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUserWithPassword(t, conn, "user1", "user1@example.com", "oldpass123")

	err := service.ChangePassword(account.ID, ChangePasswordInput{
		OldPassword:        "wrongpass123",
		NewPassword:        "newpass123",
		ConfirmNewPassword: "newpass123",
	})
	if !errors.Is(err, ErrInvalidOldPassword) {
		t.Fatalf("expected ErrInvalidOldPassword, got %v", err)
	}
}

func TestServiceChangePasswordRejectsConfirmMismatch(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUserWithPassword(t, conn, "user1", "user1@example.com", "oldpass123")

	err := service.ChangePassword(account.ID, ChangePasswordInput{
		OldPassword:        "oldpass123",
		NewPassword:        "newpass123",
		ConfirmNewPassword: "newpass456",
	})
	if !errors.Is(err, ErrPasswordConfirmationMismatch) {
		t.Fatalf("expected ErrPasswordConfirmationMismatch, got %v", err)
	}
}

func TestServiceResetPassword(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	admin := createTestUser(t, conn, "admin", "admin@example.com", RoleAdmin, "active")
	account := createTestUserWithPassword(t, conn, "user1", "user1@example.com", "oldpass123")

	err := service.ResetPassword(admin.ID, account.ID, ResetPasswordInput{
		NewPassword:        "newpass123",
		ConfirmNewPassword: "newpass123",
	})
	if err != nil {
		t.Fatalf("reset password: %v", err)
	}

	var updated User
	if err := conn.First(&updated, account.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("newpass123")); err != nil {
		t.Fatalf("expected password updated: %v", err)
	}
}

func newUserServiceForTest(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "user.db") + "?_pragma=foreign_keys(1)"
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := conn.AutoMigrate(&User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return NewService(conn), conn
}

func createTestUser(t *testing.T, conn *gorm.DB, username string, email string, role Role, status string) *User {
	t.Helper()

	return createTestUserWithHash(t, conn, username, email, role, status, mustHashPassword(t, "password123"))
}

func createTestUserWithPassword(t *testing.T, conn *gorm.DB, username string, email string, password string) *User {
	t.Helper()

	return createTestUserWithHash(t, conn, username, email, RoleUser, "active", mustHashPassword(t, password))
}

func createTestUserWithHash(t *testing.T, conn *gorm.DB, username string, email string, role Role, status string, passwordHash string) *User {
	t.Helper()

	account := &User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       status,
	}
	if err := conn.Create(account).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return account
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(hash)
}
