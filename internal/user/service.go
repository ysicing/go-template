package user

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	minPasswordLen  = 8
	statusActive    = "active"
	statusDisabled  = "disabled"
)

var (
	ErrUsernameRequired             = errors.New("username is required")
	ErrEmailRequired                = errors.New("email is required")
	ErrPasswordTooShort             = errors.New("password must be at least 8 characters")
	ErrDuplicateUsername            = errors.New("username already exists")
	ErrDuplicateEmail               = errors.New("email already exists")
	ErrInvalidRole                  = errors.New("invalid role")
	ErrInvalidStatus                = errors.New("invalid status")
	ErrCannotDisableSelf            = errors.New("cannot disable yourself")
	ErrCannotDeleteSelf             = errors.New("cannot delete yourself")
	ErrInvalidOldPassword           = errors.New("old password is incorrect")
	ErrPasswordConfirmationMismatch = errors.New("new password confirmation does not match")
)

type (
	Service struct {
		db *gorm.DB
	}

	ListUsersQuery struct {
		Keyword  string
		Role     string
		Status   string
		Page     int
		PageSize int
	}

	ListUsersResult struct {
		Items    []User `json:"items"`
		Total    int64  `json:"total"`
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
	}

	CreateUserInput struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Status   string `json:"status"`
	}

	UpdateUserInput struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Status   string `json:"status"`
	}

	ChangePasswordInput struct {
		OldPassword        string `json:"old_password"`
		NewPassword        string `json:"new_password"`
		ConfirmNewPassword string `json:"confirm_new_password"`
	}

	ResetPasswordInput struct {
		NewPassword        string `json:"new_password"`
		ConfirmNewPassword string `json:"confirm_new_password"`
	}
)

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetUser(userID uint) (*User, error) {
	var account User
	if err := s.db.First(&account, userID).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *Service) ListUsers(query ListUsersQuery) (ListUsersResult, error) {
	page, pageSize := normalizePagination(query.Page, query.PageSize)
	tx := applyListFilters(s.db.Model(&User{}), query)

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return ListUsersResult{}, err
	}

	var items []User
	err := tx.Order("id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	if err != nil {
		return ListUsersResult{}, err
	}

	return ListUsersResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) CreateUser(input CreateUserInput) (*User, error) {
	username, email, role, status := normalizeUserFields(input.Username, input.Email, input.Role, input.Status)
	password := strings.TrimSpace(input.Password)
	if err := validateIdentity(username, email); err != nil {
		return nil, err
	}
	if err := validateRoleStatus(role, status); err != nil {
		return nil, err
	}
	if len(password) < minPasswordLen {
		return nil, ErrPasswordTooShort
	}
	if err := s.ensureUnique(0, username, email); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	account := User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         Role(role),
		Status:       status,
	}
	if err := s.db.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *Service) UpdateUser(userID uint, input UpdateUserInput) (*User, error) {
	username, email, role, status := normalizeUserFields(input.Username, input.Email, input.Role, input.Status)
	if err := validateIdentity(username, email); err != nil {
		return nil, err
	}
	if err := validateRoleStatus(role, status); err != nil {
		return nil, err
	}

	var account User
	if err := s.db.First(&account, userID).Error; err != nil {
		return nil, err
	}
	if err := s.ensureUnique(userID, username, email); err != nil {
		return nil, err
	}

	account.Username = username
	account.Email = email
	account.Role = Role(role)
	account.Status = status
	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *Service) EnableUser(userID uint) error {
	return s.updateStatus(userID, statusActive)
}

func (s *Service) DisableUser(actorID uint, userID uint) error {
	if actorID == userID {
		return ErrCannotDisableSelf
	}
	return s.updateStatus(userID, statusDisabled)
}

func (s *Service) DeleteUser(actorID uint, userID uint) error {
	if actorID == userID {
		return ErrCannotDeleteSelf
	}

	var account User
	if err := s.db.First(&account, userID).Error; err != nil {
		return err
	}
	return s.db.Delete(&account).Error
}

func (s *Service) ChangePassword(userID uint, input ChangePasswordInput) error {
	normalized := normalizeChangePasswordInput(input)
	if err := validateNewPassword(normalized.NewPassword, normalized.ConfirmNewPassword); err != nil {
		return err
	}

	var account User
	if err := s.db.First(&account, userID).Error; err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(normalized.OldPassword)); err != nil {
		return ErrInvalidOldPassword
	}

	return s.updatePasswordHash(&account, normalized.NewPassword)
}

func (s *Service) ResetPassword(actorID uint, userID uint, input ResetPasswordInput) error {
	normalized := normalizeResetPasswordInput(input)
	if err := validateNewPassword(normalized.NewPassword, normalized.ConfirmNewPassword); err != nil {
		return err
	}

	var account User
	if err := s.db.First(&account, userID).Error; err != nil {
		return err
	}
	if actorID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.updatePasswordHash(&account, normalized.NewPassword)
}

func (s *Service) updateStatus(userID uint, status string) error {
	var account User
	if err := s.db.First(&account, userID).Error; err != nil {
		return err
	}
	return s.db.Model(&account).Update("status", status).Error
}

func (s *Service) ensureUnique(userID uint, username string, email string) error {
	if err := s.ensureFieldUnique(userID, "username", username, ErrDuplicateUsername); err != nil {
		return err
	}
	return s.ensureFieldUnique(userID, "email", email, ErrDuplicateEmail)
}

func (s *Service) ensureFieldUnique(userID uint, field string, value string, duplicateErr error) error {
	var count int64
	tx := s.db.Model(&User{}).Where(field+" = ?", value)
	if userID != 0 {
		tx = tx.Where("id <> ?", userID)
	}
	if err := tx.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return duplicateErr
	}
	return nil
}

func applyListFilters(tx *gorm.DB, query ListUsersQuery) *gorm.DB {
	keyword := strings.TrimSpace(query.Keyword)
	role := strings.ToLower(strings.TrimSpace(query.Role))
	status := strings.ToLower(strings.TrimSpace(query.Status))
	if keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("username LIKE ? OR email LIKE ?", like, like)
	}
	if role != "" {
		tx = tx.Where("role = ?", role)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	return tx
}

func normalizePagination(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = defaultPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

func normalizeChangePasswordInput(input ChangePasswordInput) ChangePasswordInput {
	return ChangePasswordInput{
		OldPassword:        strings.TrimSpace(input.OldPassword),
		NewPassword:        strings.TrimSpace(input.NewPassword),
		ConfirmNewPassword: strings.TrimSpace(input.ConfirmNewPassword),
	}
}

func normalizeResetPasswordInput(input ResetPasswordInput) ResetPasswordInput {
	return ResetPasswordInput{
		NewPassword:        strings.TrimSpace(input.NewPassword),
		ConfirmNewPassword: strings.TrimSpace(input.ConfirmNewPassword),
	}
}

func validateNewPassword(password string, confirmPassword string) error {
	if len(password) < minPasswordLen {
		return ErrPasswordTooShort
	}
	if confirmPassword != password {
		return ErrPasswordConfirmationMismatch
	}
	return nil
}

func (s *Service) updatePasswordHash(account *User, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(account).Update("password_hash", string(hash)).Error
}

func validateIdentity(username string, email string) error {
	if username == "" {
		return ErrUsernameRequired
	}
	if email == "" {
		return ErrEmailRequired
	}
	return nil
}

func validateRoleStatus(role string, status string) error {
	if role != string(RoleUser) && role != string(RoleAdmin) {
		return ErrInvalidRole
	}
	if status != statusActive && status != statusDisabled {
		return ErrInvalidStatus
	}
	return nil
}

func normalizeUserFields(username string, email string, role string, status string) (string, string, string, string) {
	return strings.TrimSpace(username), strings.TrimSpace(email), normalizeValue(role, string(RoleUser)), normalizeValue(status, statusActive)
}

func normalizeValue(value string, fallback string) string {
	if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
		return normalized
	}
	return fallback
}
