package conan

import "strings"

// ParseConanInfo reads a conaninfo.txt body into settings and options maps.
func ParseConanInfo(text string) (settings, options map[string]string) {
	settings = map[string]string{}
	options = map[string]string{}
	section := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		switch section {
		case "settings":
			settings[key] = value
		case "options":
			options[key] = value
		case "full_settings":
			if _, exists := settings[key]; !exists {
				settings[key] = value
			}
		case "full_options":
			if _, exists := options[key]; !exists {
				options[key] = value
			}
		}
	}
	return settings, options
}
