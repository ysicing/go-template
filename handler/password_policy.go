package handler

import "github.com/ysicing/go-template/store"

func shouldEnforcePasswordPolicy(settings settingReader) bool {
	if settings == nil {
		return true
	}
	return settings.GetBool(store.SettingPasswordPolicyEnabled, true)
}
