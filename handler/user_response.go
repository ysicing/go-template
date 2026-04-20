package handler

import (
	"time"

	"github.com/ysicing/go-template/model"
)

type UserResponse struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	IsAdmin       bool      `json:"is_admin"`
	Provider      string    `json:"provider"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	Permissions   []string  `json:"permissions"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewUserResponse(user *model.User) UserResponse {
	if user == nil {
		return UserResponse{}
	}
	permissions := user.PermissionList()
	if permissions == nil {
		permissions = []string{}
	}
	return UserResponse{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		IsAdmin:       user.IsAdmin,
		Provider:      user.Provider,
		AvatarURL:     user.AvatarURL,
		EmailVerified: user.EmailVerified,
		Permissions:   permissions,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}
