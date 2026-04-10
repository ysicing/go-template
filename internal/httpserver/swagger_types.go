package httpserver

import (
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
)

type setupStatusData struct {
	SetupRequired bool `json:"setup_required"`
}

type installResultData struct {
	Installed bool `json:"installed"`
}

type loginResponseData struct {
	User  *user.User     `json:"user"`
	Token auth.TokenPair `json:"token"`
}

type refreshResponseData struct {
	Token auth.TokenPair `json:"token"`
}

type logoutResponseData struct {
	LoggedOut bool `json:"logged_out"`
}

type changePasswordResponseData struct {
	Changed bool `json:"changed"`
}

type forgotPasswordResponseData struct {
	Sent bool `json:"sent"`
}

type resetPasswordResponseData struct {
	Changed bool `json:"changed"`
}

type currentUserResponseData struct {
	User *user.User `json:"user"`
}

type singleUserResponseData struct {
	User *user.User `json:"user"`
}

type enableUserResponseData struct {
	Enabled bool `json:"enabled"`
}

type disableUserResponseData struct {
	Disabled bool `json:"disabled"`
}

type deleteUserResponseData struct {
	Deleted bool `json:"deleted"`
}

type settingsResponseData struct {
	Items []system.Setting `json:"items"`
}

type mailSettingsResponseData struct {
	Mail system.MailSettings `json:"mail"`
}

type versionResponseData struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuildTime   string `json:"build_time"`
	FullVersion string `json:"full_version"`
}
