package maddy

import (
	"os"
)

var (
	defaultConfigDirectory  = "/etc/maddy"
	defaultStateDirectory   = "/var/lib/maddy"
	defaultLibexecDirectory = "/usr/libexec/maddy"
)

func ConfigDirectory() string {
	return defaultConfigDirectory
}

func StateDirectory(globals map[string]interface{}) string {
	if dir := os.Getenv("MADDYSTATE"); dir != "" {
		return dir
	}

	if val, ok := globals["statedir"]; ok && val.(string) != "" {
		return val.(string)
	}

	return defaultStateDirectory
}

func LibexecDirectory(globals map[string]interface{}) string {
	if dir := os.Getenv("MADDYLIBEXEC"); dir != "" {
		return dir
	}

	if val, ok := globals["libexecdir"]; ok && val.(string) != "" {
		return val.(string)
	}

	return defaultLibexecDirectory
}
