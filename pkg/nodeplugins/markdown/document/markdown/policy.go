// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"bytes"
	"io"
	"path"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
)

// Decision is the policy action for a content finding.
type Decision int

const (
	// Keep means the content passes the policy unchanged.
	Keep Decision = iota
	// StripBlock means an HTML block is replaced with a placeholder comment.
	StripBlock
	// SuppressDocument means the whole document is replaced with a placeholder.
	SuppressDocument
	// Refuse means the source is not downloaded or read.
	Refuse
	// StripHead means the "head" frontmatter key is removed.
	StripHead
	// WarnBlock means an HTML block is logged but emitted unchanged.
	WarnBlock
	// WarnNonMarkdown means a non-markdown source is logged but written unchanged.
	WarnNonMarkdown
)

// Finding records a single policy action applied to content.
// Source is used in logs only and never appears in generated output.
type Finding struct {
	Decision Decision
	Kind     string
	Source   string
}

// ContentPolicy makes stateless policy decisions over parsed content.
type ContentPolicy interface {
	BlockDecision(n *ast.HTMLBlock, src []byte) Decision
	InlineDecision(n *ast.RawHTML, src []byte) Decision
	NonMarkdownDecision(sourceURL string) Decision
}

// PolicyRenderer is a renderer.Renderer that also exposes the policy findings
// collected during the last Render call.
type PolicyRenderer interface {
	Render(w io.Writer, source []byte, n ast.Node) error
	AddOptions(opts ...renderer.Option)
	Result() []Finding
}

// DefaultPolicy is the default stateless content policy, safe to share across workers.
type DefaultPolicy struct{}

// BlockDecision returns the policy decision for an HTML block.
//
//   - Type1 block whose opening tag is exactly "script" or "style" → StripBlock.
//   - Any other block (Type1 pre/textarea, Types 2–7) that contains a script/style
//     open or close tag on any line → WarnBlock.
//   - Otherwise → Keep.
func (DefaultPolicy) BlockDecision(n *ast.HTMLBlock, src []byte) Decision {
	if n.HTMLBlockType == ast.HTMLBlockType1 {
		var first []byte
		if n.Lines().Len() > 0 {
			seg := n.Lines().At(0)
			first = seg.Value(src)
		}
		if isScriptOrStyle(htmlOpenTagName(first)) {
			return StripBlock
		}
	}
	if blockContainsScriptStyle(n, src) {
		return WarnBlock
	}
	return Keep
}

// InlineDecision returns the policy decision for inline RawHTML.
// Any segment containing a script/style open or close tag → SuppressDocument.
func (DefaultPolicy) InlineDecision(n *ast.RawHTML, src []byte) Decision {
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		if containsScriptStyleTag(seg.Value(src)) {
			return SuppressDocument
		}
	}
	return Keep
}

// NonMarkdownDecision returns the policy decision for a non-markdown source URL.
// .vue/.js/.mjs/.ts → Refuse; .html → WarnNonMarkdown; otherwise → Keep.
// Extension matching is case-insensitive.
func (DefaultPolicy) NonMarkdownDecision(sourceURL string) Decision {
	ext := strings.ToLower(path.Ext(sourceURL))
	switch ext {
	case ".vue", ".js", ".mjs", ".ts":
		return Refuse
	case ".html":
		return WarnNonMarkdown
	}
	return Keep
}

// SanitizeFrontmatter removes the "head" key from meta for untrusted nodes.
// It modifies meta only when "head" is present; otherwise it is a no-op.
// Returns a StripHead finding when the key was removed.
func SanitizeFrontmatter(meta map[string]any, trusted bool) []Finding {
	if trusted {
		return nil
	}
	if _, ok := meta["head"]; !ok {
		return nil
	}
	delete(meta, "head")
	return []Finding{{Decision: StripHead, Kind: "removed 'head' frontmatter key"}}
}

// --- tag detection helpers ---

// htmlOpenTagName extracts the lowercase opening tag name from a line.
// Returns "" if the line does not begin with "<tagname".
func htmlOpenTagName(line []byte) string {
	// Skip up to 3 leading spaces (CommonMark allows up to 3)
	i := 0
	for i < 3 && i < len(line) && line[i] == ' ' {
		i++
	}
	if i >= len(line) || line[i] != '<' {
		return ""
	}
	i++ // skip '<'
	start := i
	for i < len(line) {
		b := line[i]
		if b == ' ' || b == '\t' || b == '>' || b == '/' || b == '\n' || b == '\r' {
			break
		}
		i++
	}
	if i == start {
		return ""
	}
	return strings.ToLower(string(line[start:i]))
}

// isScriptOrStyle returns true if tag is exactly "script" or "style".
func isScriptOrStyle(tag string) bool {
	return tag == "script" || tag == "style"
}

// containsScriptStyleTag reports whether b contains a script or style open/close tag.
// Tag detection compares exact tag names: <scriptx> and <styles> do not match.
func containsScriptStyleTag(b []byte) bool {
	lower := bytes.ToLower(b)
	return containsExactTag(lower, "script") || containsExactTag(lower, "style")
}

// containsExactTag reports whether lower-cased b contains <tag or </tag where tag
// is immediately followed by whitespace, >, /, or end of data.
func containsExactTag(lower []byte, tag string) bool {
	open := []byte("<" + tag)
	close := []byte("</" + tag)
	if searchExactTag(lower, open) {
		return true
	}
	return searchExactTag(lower, close)
}

func searchExactTag(lower, prefix []byte) bool {
	haystack := lower
	for {
		idx := bytes.Index(haystack, prefix)
		if idx < 0 {
			return false
		}
		after := haystack[idx+len(prefix):]
		if isTagTerminator(after) {
			return true
		}
		haystack = haystack[idx+1:]
	}
}

func isTagTerminator(after []byte) bool {
	if len(after) == 0 {
		return true
	}
	switch after[0] {
	case '>', ' ', '\t', '\n', '\r', '/':
		return true
	}
	return false
}

// blockContainsScriptStyle scans all lines and the closure line of an HTML block.
func blockContainsScriptStyle(n *ast.HTMLBlock, src []byte) bool {
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		if containsScriptStyleTag(seg.Value(src)) {
			return true
		}
	}
	if n.HasClosure() {
		if containsScriptStyleTag(n.ClosureLine.Value(src)) {
			return true
		}
	}
	return false
}
