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

// convertJSPathToXPath converts JSPath to xp in XPath format.
func convertJSPathToXPath(jsPath string) string {
	if jsPath == "" {
		return ""
	}

	p := replaceAllIgnoreKeys(jsPath, "_", "-")

	// Replace {.name=="key"} with [name=key]; List nodes
	var sb strings.Builder
	sb.Grow(len(p) + 10) // Pre-allocate some extra space for potential additions

	// Iterate two characters at a time
	for i := 0; i < len(p)-1; i++ {
		str := p[i : i+2]
		switch str {
		case "{.":
			sb.WriteString("[")
			i++
		case "\"}":
			sb.WriteString("]")
			i++
		case "==":
			sb.WriteString("=")
			i += 2 // skip \" char in "==\""
		default:
			sb.WriteByte(str[0])
			// write last char if second to last index
			if i == len(p)-2 {
				sb.WriteByte(str[1])
			}
		}
	}

	return replaceAllIgnoreKeys(sb.String(), ".", "/")
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
