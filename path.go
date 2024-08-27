package bond

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	ignoreKeysPattern = `\[.*?\]|(%s)`
)

// convertXPathToJSPath converts xp in XPath format to JSPath.
func convertXPathToJSPath(xp string) string {
	if xp == "" {
		return ""
	}

	p := replaceAllIgnoreKeys(xp, "/", ".")
	p = replaceAllIgnoreKeys(p, "-", "_")

	// Replace [name=key] with {.name=="key"}; List nodes
	var sb strings.Builder
	sb.Grow(len(xp) + 10) // Pre-allocate some extra space for potential additions

	for _, ch := range p {
		switch ch {
		case '[':
			sb.WriteString("{.")
		case ']':
			sb.WriteString("\"}")
		case '=':
			sb.WriteString("==\"")
		default:
			sb.WriteRune(ch)
		}
	}

	return sb.String()
}

// replaceAllIgnoreKeys replaces oldStr substring in path with newStr.
// list keys in brackets that contain oldStr are not replaced.
// e.g. /ndkDemo/list-node[ethernet-1/1], "/", "." -> .ndkDemo.list-node[ethernet-1/1]
func replaceAllIgnoreKeys(path, oldStr, newStr string) string {
	// Compile the regex pattern
	pattern := fmt.Sprintf(ignoreKeysPattern, regexp.QuoteMeta(oldStr))
	re := regexp.MustCompile(pattern)

	// Perform the replacement
	result := re.ReplaceAllStringFunc(path, func(match string) string {
		if match == oldStr {
			return newStr
		}
		return match
	})

	return result
}
