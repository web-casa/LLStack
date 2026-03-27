package php

import (
	"fmt"
	"strings"
)

// RenderProfile renders the managed php.ini snippet for a profile.
func RenderProfile(profile ProfileName) (string, error) {
	switch profile {
	case "", ProfileGeneric:
		return renderINI(map[string]string{
			"memory_limit":        "256M",
			"upload_max_filesize": "64M",
			"post_max_size":       "64M",
			"max_execution_time":  "60",
			"expose_php":          "Off",
		}), nil
	case ProfileWP:
		return renderINI(map[string]string{
			"memory_limit":        "512M",
			"upload_max_filesize": "128M",
			"post_max_size":       "128M",
			"max_execution_time":  "120",
			"max_input_vars":      "5000",
			"expose_php":          "Off",
		}), nil
	case ProfileLaravel:
		return renderINI(map[string]string{
			"memory_limit":                "512M",
			"upload_max_filesize":         "64M",
			"post_max_size":               "64M",
			"max_execution_time":          "120",
			"opcache.validate_timestamps": "1",
			"expose_php":                  "Off",
		}), nil
	case ProfileAPI:
		return renderINI(map[string]string{
			"memory_limit":       "256M",
			"max_execution_time": "30",
			"max_input_vars":     "2000",
			"expose_php":         "Off",
			"display_errors":     "Off",
		}), nil
	case ProfileCustom:
		return "; custom profile reserved for user-managed overrides\n", nil
	default:
		return "", fmt.Errorf("unsupported php profile %q", profile)
	}
}

func renderINI(values map[string]string) string {
	order := []string{
		"memory_limit",
		"upload_max_filesize",
		"post_max_size",
		"max_execution_time",
		"max_input_vars",
		"opcache.validate_timestamps",
		"display_errors",
		"expose_php",
	}
	lines := []string{"; Managed by LLStack"}
	for _, key := range order {
		if value, ok := values[key]; ok {
			lines = append(lines, fmt.Sprintf("%s = %s", key, value))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
