package bash

import "regexp"

// promptPatterns matches common interactive-REPL prompts anchored to the end
// of the accumulated output. When one matches, the process is almost
// certainly sitting idle waiting for stdin - there's no point waiting out
// the rest of a fixed timeout to find that out, and no point returning
// output that's mid-line (not yet followed by a prompt) as if it were done.
//
// Deliberately conservative: every pattern requires trailing whitespace-only
// content after the prompt token (`$` anchors to end-of-string), so a prompt
// string that merely appears mid-output (e.g. a REPL banner mentioning ">>>")
// doesn't false-positive. This mirrors DesktopCommanderMCP's
// analyzeProcessState, which uses the same anchor-to-end approach.
var promptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`>>>\s*$`),         // Python
	regexp.MustCompile(`\.\.\.\s*$`),      // Python/Node continuation
	regexp.MustCompile(`In \[\d+\]:\s*$`), // IPython/Jupyter console
	regexp.MustCompile(`>\s*$`),           // Node.js REPL, R, generic
	regexp.MustCompile(`julia>\s*$`),      // Julia
	regexp.MustCompile(`\(gdb\)\s*$`),     // GDB
	regexp.MustCompile(`mysql>\s*$`),      // MySQL
	regexp.MustCompile(`\s*->\s*$`),       // MySQL continuation
	regexp.MustCompile(`[=\-][#>]\s*$`),   // psql (=# -# => -> variants)
	regexp.MustCompile(`\$\s*$`),          // generic shell
	regexp.MustCompile(`#\s*$`),           // generic shell (root)
	regexp.MustCompile(`Password.*:\s*$`), // password prompts
	regexp.MustCompile(`\?\s*$`),          // y/n or "Continue?" style prompts
	regexp.MustCompile(`\[y/[nN]\]\s*:?\s*$`),
}

// looksLikeAwaitingInput reports whether output's tail matches a known
// interactive-prompt shape. Only the trailing window is checked (not the
// whole buffer) since that's both cheaper and far less prone to matching an
// incidental prompt-like substring buried earlier in the output.
func looksLikeAwaitingInput(output string) bool {
	const tailWindow = 200
	tail := output
	if len(tail) > tailWindow {
		tail = tail[len(tail)-tailWindow:]
	}
	if tail == "" {
		return false
	}
	for _, p := range promptPatterns {
		if p.MatchString(tail) {
			return true
		}
	}
	return false
}
