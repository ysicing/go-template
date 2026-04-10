package system

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SettingSiteName         = "site.name"
	SettingMailEnabled      = "mail.enabled"
	SettingMailSMTPHost     = "mail.smtp_host"
	SettingMailSMTPPort     = "mail.smtp_port"
	SettingMailUsername     = "mail.username"
	SettingMailPassword     = "mail.password"
	SettingMailFrom         = "mail.from"
	SettingMailResetBaseURL = "mail.reset_base_url"
)

type MailSettings struct {
	Enabled      bool   `json:"enabled"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	From         string `json:"from"`
	ResetBaseURL string `json:"reset_base_url"`
	PasswordSet  bool   `json:"password_set"`
	SiteName     string `json:"site_name"`
}

func DefaultMailSettings() MailSettings {
	return MailSettings{
		Enabled:  false,
		SMTPPort: 587,
		SiteName: "Go Template",
	}
}

func LoadMailSettings(conn *gorm.DB) (MailSettings, error) {
	settings := DefaultMailSettings()
	values, err := loadSettingMap(conn, []string{
		SettingSiteName,
		SettingMailEnabled,
		SettingMailSMTPHost,
		SettingMailSMTPPort,
		SettingMailUsername,
		SettingMailPassword,
		SettingMailFrom,
		SettingMailResetBaseURL,
	})
	if err != nil {
		return MailSettings{}, err
	}

	settings.SiteName = normalizeString(values[SettingSiteName], settings.SiteName)
	settings.Enabled = normalizeBool(values[SettingMailEnabled], false)
	settings.SMTPHost = strings.TrimSpace(values[SettingMailSMTPHost])
	settings.SMTPPort = normalizeInt(values[SettingMailSMTPPort], settings.SMTPPort)
	settings.Username = strings.TrimSpace(values[SettingMailUsername])
	settings.Password = values[SettingMailPassword]
	settings.From = strings.TrimSpace(values[SettingMailFrom])
	settings.ResetBaseURL = strings.TrimRight(strings.TrimSpace(values[SettingMailResetBaseURL]), "/")
	settings.PasswordSet = strings.TrimSpace(settings.Password) != ""

	return settings, nil
}

func SaveMailSettings(conn *gorm.DB, input MailSettings) error {
	items := []Setting{
		{Group: "mail", Key: SettingMailEnabled, Value: strconv.FormatBool(input.Enabled)},
		{Group: "mail", Key: SettingMailSMTPHost, Value: strings.TrimSpace(input.SMTPHost)},
		{Group: "mail", Key: SettingMailSMTPPort, Value: strconv.Itoa(input.SMTPPort)},
		{Group: "mail", Key: SettingMailUsername, Value: strings.TrimSpace(input.Username)},
		{Group: "mail", Key: SettingMailFrom, Value: strings.TrimSpace(input.From)},
		{Group: "mail", Key: SettingMailResetBaseURL, Value: strings.TrimRight(strings.TrimSpace(input.ResetBaseURL), "/")},
	}
	if strings.TrimSpace(input.Password) != "" {
		items = append(items, Setting{Group: "mail", Key: SettingMailPassword, Value: input.Password})
	}

	for index := range items {
		if err := conn.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"group", "value", "updated_at"}),
		}).Create(&items[index]).Error; err != nil {
			return err
		}
	}

	return nil
}

func MailSettingsConfigured(settings MailSettings) bool {
	return settings.Enabled &&
		strings.TrimSpace(settings.SMTPHost) != "" &&
		settings.SMTPPort > 0 &&
		strings.TrimSpace(settings.From) != "" &&
		strings.TrimSpace(settings.ResetBaseURL) != ""
}

func loadSettingMap(conn *gorm.DB, keys []string) (map[string]string, error) {
	var items []Setting
	if err := conn.Where("key IN ?", keys).Find(&items).Error; err != nil {
		return nil, err
	}
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.Key] = item.Value
	}
	return values, nil
}

func normalizeBool(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeInt(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
