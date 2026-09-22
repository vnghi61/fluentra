package rendition

import (
	"fmt"
	"strings"
)

// The arguments every ffmpeg and ffprobe call carries.
const (
	ProtocolWhitelistArg = "-protocol_whitelist"
	ProtocolWhitelistVal = "file"
	NoStdinArg           = "-nostdin"
)

// BuildFFmpegArgs wraps ffmpeg arguments to guarantee required security limits:
// specifically, -nostdin and -protocol_whitelist file to prevent SSRF and hanging.
func BuildFFmpegArgs(customArgs ...string) []string {
	// Prepend -nostdin and -protocol_whitelist file
	args := []string{NoStdinArg, ProtocolWhitelistArg, ProtocolWhitelistVal}
	args = append(args, customArgs...)
	return args
}

// BuildFFprobeArgs wraps ffprobe arguments ensuring -protocol_whitelist file.
func BuildFFprobeArgs(customArgs ...string) []string {
	args := []string{ProtocolWhitelistArg, ProtocolWhitelistVal}
	args = append(args, customArgs...)
	return args
}

// ValidateFFmpegArgs inspects an argument list to assert that -protocol_whitelist file
// is explicitly present and cannot be dropped.
func ValidateFFmpegArgs(args []string) error {
	foundWhitelist := false
	for i, arg := range args {
		if arg == ProtocolWhitelistArg {
			if i+1 < len(args) && args[i+1] == ProtocolWhitelistVal {
				foundWhitelist = true
				break
			}
			if i+1 < len(args) && strings.Contains(args[i+1], ProtocolWhitelistVal) {
				foundWhitelist = true
				break
			}
		}
	}
	if !foundWhitelist {
		return fmt.Errorf("security violation: %s %s is missing from ffmpeg arguments",
			ProtocolWhitelistArg, ProtocolWhitelistVal)
	}
	return nil
}
