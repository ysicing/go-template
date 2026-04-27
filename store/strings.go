package store

import "strings"

// SplitTrimmed 将逗号或空白分隔的字符串切分后去除空项。
func SplitTrimmed(s string) []string {
	if s == "" {
		return nil
	}
	normalized := strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(normalized)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
