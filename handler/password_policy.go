package handler

import "github.com/ysicing/go-template/store"

func shouldEnforcePasswordPolicy(settings settingReader) bool {
	if settings == nil {
		return false
	}
	return settings.GetBool(store.SettingPasswordPolicyEnabled, false)
}
