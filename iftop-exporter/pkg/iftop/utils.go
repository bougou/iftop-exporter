package iftop

import (
	"regexp"
	"strings"
)

// GetNamedCapturingGroupMap returns a map holding the named capturing group information
// for the input string by using the specified regexp pattern matcher.
//
// The first parameter is normally created by  `reg := regexp.MustCompile(somePatternStr)`
func GetNamedCapturingGroupMap(matcher *regexp.Regexp, input string) (result map[string]string, matched bool) {
	if matcher == nil {
		return nil, false
	}

	match := matcher.FindStringSubmatch(input)
	if match == nil {
		return nil, false
	}

	result = make(map[string]string)

	if len(match) == 0 {
		return result, true
	}

	for i, name := range matcher.SubexpNames() {
		if i == 0 {
			continue
		}
		if i >= len(match) {
			break
		}
		result[name] = match[i]
	}

	return result, true
}

// the input addr may contain port
func extractIP(addr string) string {
	if addr == "" {
		return ""
	}

	// [IPv6]:Port
	if strings.Contains(addr, "]:") {
		parts := strings.Split(addr, "]:")

		if len(parts) > 0 {
			s, _ := strings.CutPrefix(parts[0], "[")
			return s
		}
		return ""
	}

	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			// IPv4:Port
			return parts[0]
		}

		// IPv6
		return addr
	}

	// IPv4
	return addr
}
