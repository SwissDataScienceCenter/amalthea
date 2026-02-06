package utils

import "strings"

const RemoteSessionControllerPrefix = "RSC"

// Converts a flag into its environment variable version, with the "RSC" prefix.
// Example: my-flag -> RSC_MY_FLAG
// NOTE: "RSC" stands for "Remote Session Controller"
func AsEnvVarFlag(flag string) string {
	withUnderscores := strings.ReplaceAll(RemoteSessionControllerPrefix+"_"+flag, "-", "_")
	return strings.ToUpper(withUnderscores)
}
