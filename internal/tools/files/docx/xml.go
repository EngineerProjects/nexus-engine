package docx

import "strings"

// prettyPrintXML splits compact XML onto one line per tag, indented by
// nesting depth, so old_string/new_string find/replace has readable,
// addressable text to match against instead of one giant minified line.
// compactXML reverses it losslessly: compact -> pretty -> compact round-trips
// exactly, since indentation is pure leading whitespace added between tags,
// never inside a text node.
//
// Mirrors DesktopCommanderMCP's docx.ts prettyPrintXml/compactXml exactly -
// same algorithm, since the round-trip losslessness is what makes it safe to
// edit and repack, and there's no reason to redesign a working scheme.
func prettyPrintXML(xml string) string {
	parts := splitAdjacentTags(xml)
	var lines []string
	depth := 0

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		isClosing := strings.HasPrefix(trimmed, "</")
		isSelfClosing := strings.HasSuffix(trimmed, "/>")
		isProcessingInstruction := strings.HasPrefix(trimmed, "<?")
		isInline := !isClosing && !isSelfClosing && strings.Contains(trimmed, "</")

		if isClosing && depth > 0 {
			depth--
		}

		lines = append(lines, strings.Repeat("  ", depth)+trimmed)

		if !isClosing && !isSelfClosing && !isInline && !isProcessingInstruction {
			depth++
		}
	}

	return strings.Join(lines, "\n")
}

// splitAdjacentTags splits xml at every ">less-than<" boundary, i.e.
// between one tag's end and the next tag's start, mirroring
// xml.split(/(?<=>)(?=<)/) - Go's regexp package has no lookaround, so this
// walks the string directly instead.
func splitAdjacentTags(xml string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(xml)-1; i++ {
		if xml[i] == '>' && xml[i+1] == '<' {
			parts = append(parts, xml[start:i+1])
			start = i + 1
		}
	}
	parts = append(parts, xml[start:])
	return parts
}

// compactXML strips each line's leading indentation and rejoins - it does
// not touch whitespace inside text node content, since that whitespace is
// never at the start of a pretty-printed line (a line always starts with a
// tag or with text immediately following one).
func compactXML(prettyXML string) string {
	lines := strings.Split(prettyXML, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeft(l, " ")
	}
	return strings.Join(lines, "")
}
