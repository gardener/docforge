// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
	"io"
	"regexp"
	"sync"
)

var (
	// pool with reusable buffers
	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	// defines an ordered list item marker without next not space char '[^ ]+'
	marker = regexp.MustCompile(`^\d{1,9}[.)] {1,4}`)
	// defines a fence block line
	fence = regexp.MustCompile("^ {0,3}```")
	// GFM autolink extensions
	http  = regexp.MustCompile(`^https?://(?:[a-zA-Z\d\-_]+\.)*[a-zA-Z\d\-]+\.[a-zA-Z\d\-]+[^ <]*$`)
	www   = regexp.MustCompile(`^www\.(?:[a-zA-Z\d\-_]+\.)*[a-zA-Z\d\-]+\.[a-zA-Z\d\-]+[^ <]*$`)
	email = regexp.MustCompile(`^[a-zA-Z\d.\-+]+@(?:[a-zA-Z\d\-_]+\.)+[a-zA-Z\d\-_]+$`)
)

// ResolveLink type defines function for modifying link destination
// dest - original destination
// isEmbeddable - if true, raw destination required
type ResolveLink func(dest string, isEmbeddable bool) (string, error)

// resolveSame implements markdown.ResolveLink - the result is the same as input
// used if WithLinkResolver option is not set
func resolveSame(dest string, _ bool) (string, error) {
	return dest, nil
}

// LinkResolver is an option name used in WithLinkResolver.
const optLinkResolver renderer.OptionName = "LinkResolver"

type withLinkResolver struct {
	value ResolveLink
}

func (o *withLinkResolver) SetConfig(c *renderer.Config) {
	c.Options[optLinkResolver] = o.value
}

// WithLinkResolver is a functional option that allow you to set the ResolveLink to the renderer.
func WithLinkResolver(linkResolver ResolveLink) renderer.Option {
	return &withLinkResolver{linkResolver}
}

// A linkModifierRenderer struct is an implementation of renderer.Renderer interface.
type linkModifierRenderer struct {
	config *renderer.Config
}

// NewLinkModifierRenderer returns a new linkModifierRenderer with given renderer options.
func NewLinkModifierRenderer(opts ...renderer.Option) renderer.Renderer {
	config := renderer.NewConfig()
	for _, opt := range opts {
		opt.SetConfig(config)
	}
	if _, ok := config.Options[optLinkResolver]; !ok {
		WithLinkResolver(resolveSame).SetConfig(config)
	}
	return &linkModifierRenderer{
		config: config,
	}
}

func (l *linkModifierRenderer) AddOptions(opts ...renderer.Option) {
	for _, opt := range opts {
		opt.SetConfig(l.config)
	}
}

func (l *linkModifierRenderer) Render(w io.Writer, source []byte, node ast.Node) error {
	// walk & render nodes
	r := &Renderer{
		source:       source,
		linkResolver: l.config.Options[optLinkResolver].(ResolveLink),
		indents:      make([]byte, 0, 20),
		markers:      make([]int, 0, 5),
		emphasis:     make([]byte, 0, 5),
	}
	writer, ok := w.(*bytes.Buffer)
	if ok {
		r.writer = writer
	} else {
		r.writer = &bytes.Buffer{}
	}
	err := ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		switch node.Kind() {
		case ast.KindDocument:
			return r.renderDocument(node, entering)
		// commonmark container blocks
		case ast.KindBlockquote:
			return r.renderBlockquote(node, entering)
		case ast.KindList:
			return r.renderList(node, entering)
		case ast.KindListItem:
			return r.renderListItem(node, entering)
		// commonmark blocks
		case ast.KindHeading:
			return r.renderHeading(node, entering)
		case ast.KindCodeBlock, ast.KindFencedCodeBlock:
			return r.renderFencedCodeBlock(node, entering)
		case ast.KindHTMLBlock:
			return r.renderHTMLBlock(node, entering)
		case ast.KindParagraph:
			return r.renderParagraph(node, entering)
		case ast.KindTextBlock:
			return r.renderTextBlock(node, entering)
		case ast.KindThematicBreak:
			return r.renderThematicBreak(node, entering)
		// commonmark inlines
		case ast.KindAutoLink:
			return r.renderAutoLink(node, entering)
		case ast.KindCodeSpan:
			return r.renderCodeSpan(node, entering)
		case ast.KindEmphasis:
			return r.renderEmphasis(node, entering)
		case ast.KindLink:
			return r.renderLink(node, entering)
		case ast.KindImage:
			return r.renderImage(node, entering)
		case ast.KindRawHTML:
			return r.renderRawHTML(node, entering)
		case ast.KindText, ast.KindString:
			return r.renderText(node, entering)
		// GFM extension blocks
		case extast.KindTable:
			return r.renderTable(node, entering)
		case extast.KindTableHeader:
			return r.renderTableHeader(node, entering)
		case extast.KindTableRow:
			return r.renderTableRow(node, entering)
		case extast.KindTableCell:
			return r.renderTableCell(node, entering)
		// GFM extension inlines
		case extast.KindTaskCheckBox:
			return r.renderTaskCheckBox(node, entering)
		case extast.KindStrikethrough:
			return r.renderStrikethrough(node, entering)
		default:
			return ast.WalkContinue, nil
		}
	})
	if !ok {
		_, _ = w.Write(r.writer.Bytes())
	}
	return err
}

// Renderer holds document source, buffer writer, info for indents and some nodes for rendering a markdown
type Renderer struct {
	source       []byte
	writer       *bytes.Buffer
	linkResolver ResolveLink
	indents      []byte
	markers      []int
	emphasis     []byte
	table        bool
}

// --------------------------- Node Renders

func (r *Renderer) renderDocument(node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Document)
	if entering {
		var err error
		// process frontmatter
		fm := n.Meta()
		if len(fm) > 0 {
			_, _ = r.writer.Write([]byte("---\n"))
			var cnt []byte
			cnt, err = yaml.Marshal(fm)
			if err != nil {
				return ast.WalkStop, err
			}
			_, _ = r.writer.Write(cnt)
			_, _ = r.writer.Write([]byte("---\n"))
			if n.HasChildren() {
				r.newLine(false)
			}
		}
	} else {
		if n.HasChildren() {
			cnt := r.writer.Bytes()
			if len(cnt) > 0 && cnt[len(cnt)-1] != '\n' {
				r.newLine(false)
			}
		}
	}
	return ast.WalkContinue, nil
}

// commonmark container blocks

func (r *Renderer) renderBlockquote(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.blockSeparator(n)
		// no laziness - block new lines will always start with '>'
		_, _ = r.writer.Write([]byte("> "))
		r.indents = append(r.indents, '>', ' ')
	} else {
		r.indents = r.indents[:len(r.indents)-2]
		breakBlockquoteLazyContinuation(n.NextSibling())
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderList(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.blockSeparator(n)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderListItem(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.ListItem)
		r.blockSeparator(n)
		listMarker := buildListMarker(n)
		_, _ = r.writer.Write(listMarker)
		r.markers = append(r.markers, len(listMarker))
		r.indents = append(r.indents, bytes.Repeat([]byte{' '}, len(listMarker))...)
	} else {
		r.indents = r.indents[:len(r.indents)-r.markers[len(r.markers)-1]]
		r.markers = r.markers[:len(r.markers)-1]
	}
	return ast.WalkContinue, nil
}

// commonmark blocks

func (r *Renderer) renderHeading(node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	atx := true // defaults to ATX headings
	if n.Lines().Len() > 1 && n.Level <= 2 {
		atx = false // multiline heading -> use Setext headings
	}
	if entering {
		r.blockSeparator(n)
		if atx {
			_, _ = r.writer.Write(bytes.Repeat([]byte{'#'}, n.Level))
			_ = r.writer.WriteByte(' ')
		}
	} else {
		if !atx {
			r.newLine(true)
			if n.Level == 1 {
				_, _ = r.writer.Write([]byte{'=', '=', '='})
			} else {
				_, _ = r.writer.Write([]byte{'-', '-', '-'})
			}
		}
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderFencedCodeBlock(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		buf := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(buf)
		buf.Reset()
		indents := len(r.indents) > 0
		var fb byte = '`'
		segments := n.Lines()
		for _, l := range segments.Sliced(0, segments.Len()) {
			if fence.Match(l.Value(r.source)) {
				fb = '~'
			}
			if indents {
				_, _ = buf.Write(r.indents)
			}
			_, _ = buf.Write(l.Value(r.source))
		}
		r.blockSeparator(n)
		_, _ = r.writer.Write([]byte{fb, fb, fb})
		if n.Kind() == ast.KindFencedCodeBlock {
			fn := n.(*ast.FencedCodeBlock)
			language := fn.Language(r.source)
			if language != nil {
				_, _ = r.writer.Write(language)
			}
		}
		r.newLine(false)
		if buf.Len() > 0 {
			_, _ = r.writer.Write(buf.Bytes())
		}
		if indents {
			_, _ = r.writer.Write(r.indents)
		}
		_, _ = r.writer.Write([]byte{fb, fb, fb})
	}
	return ast.WalkSkipChildren, nil
}

func (r *Renderer) renderHTMLBlock(node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.HTMLBlock)
	if entering {
		r.blockSeparator(n)
		// HTMLBlockType 6 & 7 may contain links and images
		if n.HTMLBlockType >= ast.HTMLBlockType6 {
			buf := bufPool.Get().(*bytes.Buffer)
			defer bufPool.Put(buf)
			buf.Reset()
			r.writeSegments(buf, n.Lines(), false)
			// modify
			modBuf := bufPool.Get().(*bytes.Buffer)
			defer bufPool.Put(modBuf)
			modBuf.Reset()
			modified, err := r.modifyHTMLTags(buf.Bytes(), modBuf)
			if err != nil {
				return ast.WalkStop, err
			}
			if modified {
				buf = modBuf
			}
			r.writeContent(buf.Bytes())
		} else {
			r.writeSegments(r.writer, n.Lines(), len(r.indents) > 0)
			// HTMLBlockType 1 to 5 end condition is not blank line
			if n.HasClosure() {
				// line that contains end condition for blocks with type < 6
				if len(r.indents) > 0 {
					_, _ = r.writer.Write(r.indents)
				}
				_, _ = r.writer.Write(n.ClosureLine.Value(r.source))
			}
		}
	}
	return ast.WalkSkipChildren, nil
}

func (r *Renderer) renderParagraph(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.blockSeparator(n)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderTextBlock(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.blockSeparator(n)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderThematicBreak(node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.ThematicBreak)
	if entering {
		r.blockSeparator(n)
		if n.HasBlankPreviousLines() {
			_, _ = r.writer.Write([]byte{'-', '-', '-'})
		} else {
			// as '-' could be Setext heading 2 use '*'
			_, _ = r.writer.Write([]byte{'*', '*', '*'})
		}
	}
	return ast.WalkSkipChildren, nil
}

// commonmark inlines

func (r *Renderer) renderAutoLink(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.AutoLink)
		label := n.Label(r.source)
		if bytes.HasPrefix(bytes.ToLower(label), []byte("mailto:")) {
			n.AutoLinkType = ast.AutoLinkEmail // fix the node type
		}
		classic := isClassicAutolink(label, r.writer.Bytes())
		if classic {
			_ = r.writer.WriteByte('<')
		}
		if n.AutoLinkType == ast.AutoLinkURL {
			dest, err := r.linkResolver(string(label), false)
			if err != nil {
				return ast.WalkStop, err
			}
			_, _ = r.writer.Write([]byte(dest))
		} else {
			_, _ = r.writer.Write(label)
		}
		if classic {
			_ = r.writer.WriteByte('>')
		}
	}
	return ast.WalkSkipChildren, nil
}

func (r *Renderer) renderCodeSpan(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		cs := []byte{'`'}
		txt := n.Text(r.source)
		c := bytes.Count(txt, []byte{'`'}) // if odd backtick count use "``" as code span string
		if c%2 == 0 {
			if n.PreviousSibling() != nil {
				idx := bytes.LastIndexByte(r.writer.Bytes(), '\n')
				if idx == -1 {
					idx = 0
				}
				c = bytes.Count(r.writer.Bytes()[idx:], []byte{'`'}) // if previous text has odd backtick count use "``"
			}
		}
		if c%2 != 0 {
			cs = append(cs, '`')
		}
		// if text starts or ends with '`' or ' ' add space
		space := len(txt) > 0 && (txt[0] == '`' || txt[0] == ' ' || txt[len(txt)-1] == '`' || txt[len(txt)-1] == ' ')
		if space && len(txt) == bytes.Count(txt, []byte{' '}) {
			// if txt contains only spaces don't add additional space
			space = false
		}
		_, _ = r.writer.Write(cs)
		if space {
			_ = r.writer.WriteByte(' ')
		}
		if r.table { // parser unescape '|' in code span
			txt = escapePipes(txt)
		}
		txt = bytes.ReplaceAll(txt, []byte{'\n'}, []byte{' '}) // replace new lines with spaces
		_, _ = r.writer.Write(txt)
		if space {
			_ = r.writer.WriteByte(' ')
		}
		_, _ = r.writer.Write(cs)
	}
	return ast.WalkSkipChildren, nil
}

func (r *Renderer) renderEmphasis(node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Emphasis)
	if entering {
		ch, _ := r.calcEmphasisChar(n)
		_, _ = r.writer.Write(bytes.Repeat([]byte{ch}, n.Level))
		r.emphasis = append(r.emphasis, ch)
	} else {
		_, _ = r.writer.Write(bytes.Repeat([]byte{r.emphasis[len(r.emphasis)-1]}, n.Level))
		r.emphasis = r.emphasis[:len(r.emphasis)-1]
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderLink(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_ = r.writer.WriteByte('[')
	} else {
		n := node.(*ast.Link)
		_ = r.writer.WriteByte(']')
		_ = r.writer.WriteByte('(')
		dest, err := r.linkResolver(string(n.Destination), false)
		if err != nil {
			return ast.WalkStop, err
		}
		wrap := wrapLinkDestination([]byte(dest))
		if wrap {
			_ = r.writer.WriteByte('<')
		}
		_, _ = r.writer.Write([]byte(dest))
		if wrap {
			_ = r.writer.WriteByte('>')
		}
		if n.Title != nil {
			q := getLinkTitleWrapper(n.Title)
			_ = r.writer.WriteByte(' ')
			_ = r.writer.WriteByte(q)
			r.writeContent(n.Title) // support multi-line title
			if q == '(' {
				q = ')'
			}
			_ = r.writer.WriteByte(q)
		}
		_ = r.writer.WriteByte(')')
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderImage(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_ = r.writer.WriteByte('!')
		_ = r.writer.WriteByte('[')
	} else {
		n := node.(*ast.Image)
		_ = r.writer.WriteByte(']')
		_ = r.writer.WriteByte('(')
		dest, err := r.linkResolver(string(n.Destination), true)
		if err != nil {
			return ast.WalkStop, err
		}
		wrap := wrapLinkDestination([]byte(dest))
		if wrap {
			_ = r.writer.WriteByte('<')
		}
		_, _ = r.writer.Write([]byte(dest))
		if wrap {
			_ = r.writer.WriteByte('>')
		}
		if n.Title != nil {
			q := getLinkTitleWrapper(n.Title)
			_ = r.writer.WriteByte(' ')
			_ = r.writer.WriteByte(q)
			r.writeContent(n.Title) // support multi-line title
			if q == '(' {
				q = ')'
			}
			_ = r.writer.WriteByte(q)
		}
		_ = r.writer.WriteByte(')')
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderRawHTML(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.RawHTML)
		buf := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(buf)
		buf.Reset()
		r.writeSegments(buf, n.Segments, false)
		// modify
		modBuf := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(modBuf)
		modBuf.Reset()
		modified, err := r.modifyHTMLTags(buf.Bytes(), modBuf)
		if err != nil {
			return ast.WalkStop, err
		}
		if modified {
			buf = modBuf
		}
		r.writeContent(buf.Bytes())
	}
	return ast.WalkSkipChildren, nil
}

func (r *Renderer) renderText(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.Text)
		txt := n.Text(r.source)
		r.additionalIndents(txt, n)
		if n.HardLineBreak() || n.SoftLineBreak() || nextIsLineBreak(node.NextSibling(), r.source) {
			// trim trailing spaces
			txt = bytes.TrimRight(txt, " ")
		}
		_, _ = r.writer.Write(txt)
		indents := len(r.indents) > 0
		if n.HardLineBreak() {
			_ = r.writer.WriteByte(' ')
			_ = r.writer.WriteByte(' ')
			r.newLine(indents)
		} else if n.SoftLineBreak() {
			r.newLine(indents)
		}
	}
	return ast.WalkSkipChildren, nil
}

// GFM extension blocks

func (r *Renderer) renderTable(n ast.Node, entering bool) (ast.WalkStatus, error) {
	// https://pkg.go.dev/text/tabwriter - for pretty table writing
	if entering {
		// 'blankPreviousLines' is not propagated during transformations, so previous blank line is set
		n.SetBlankPreviousLines(true)
		r.blockSeparator(n)
		r.table = true
	} else {
		r.table = false
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderTableHeader(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		n := node.(*extast.TableHeader)
		if n.Alignments == nil {
			if t, ok := n.Parent().(*extast.Table); ok {
				n.Alignments = t.Alignments
			}
		}
		// close headers cells
		_ = r.writer.WriteByte('|')
		r.newLine(len(r.indents) > 0)
		// write alignments
		for _, a := range n.Alignments {
			_ = r.writer.WriteByte('|')
			align := []byte(" --- ")
			switch a {
			case extast.AlignLeft:
				align = []byte(" :-- ")
			case extast.AlignRight:
				align = []byte(" --: ")
			case extast.AlignCenter:
				align = []byte(" :-: ")
			}
			_, _ = r.writer.Write(align)
		}
		_ = r.writer.WriteByte('|')
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderTableRow(_ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.newLine(len(r.indents) > 0)
	} else {
		_ = r.writer.WriteByte('|')
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderTableCell(_ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_ = r.writer.WriteByte('|')
		_ = r.writer.WriteByte(' ')
	} else {
		_ = r.writer.WriteByte(' ')
	}
	return ast.WalkContinue, nil
}

// GFM extension inlines

func (r *Renderer) renderTaskCheckBox(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*extast.TaskCheckBox)
		if n.IsChecked {
			_, _ = r.writer.Write([]byte("[X] "))
		} else {
			_, _ = r.writer.Write([]byte("[ ] "))
		}
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderStrikethrough(_ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = r.writer.Write([]byte("~~"))
	} else {
		_, _ = r.writer.Write([]byte("~~"))
	}
	return ast.WalkContinue, nil
}

// ---------------------------

func (r *Renderer) newLine(indents bool) {
	_ = r.writer.WriteByte('\n')
	if indents {
		_, _ = r.writer.Write(r.indents)
	}
}

// separates blocks
func (r *Renderer) blockSeparator(n ast.Node) {
	if n.PreviousSibling() != nil {
		// add new line to start a blocks
		// for some blocks like HTMLBlock content is raw and ends with '\n', so skip new line in such case
		cnt := r.writer.Bytes()
		if len(cnt) > 0 && cnt[len(cnt)-1] != '\n' {
			_ = r.writer.WriteByte('\n')
		}
		// add indents after new line
		if len(r.indents) > 0 {
			_, _ = r.writer.Write(r.indents)
		}
		// add a blank line between blocks in case there are blank previous lines
		// in blockquote scope 'blankPreviousLines' flag is not calculated properly (always `false`), so blank line
		// should be added in some cases
		if n.HasBlankPreviousLines() || r.blankLineInBlockquoteScope(n) {
			_ = r.writer.WriteByte('\n')
			if len(r.indents) > 0 {
				_, _ = r.writer.Write(r.indents)
			}
		}
	}
}

// returns true if blank line should be added between Text inlines or Paragraph|TestBlock blocks in blockquote scope
func (r *Renderer) blankLineInBlockquoteScope(n ast.Node) bool {
	if bytes.IndexByte(r.indents, '>') != -1 && n.PreviousSibling() != nil {
		k := n.Kind()
		if k == ast.KindText || k == ast.KindParagraph || k == ast.KindTextBlock {
			pk := n.PreviousSibling().Kind()
			return pk == ast.KindText || pk == ast.KindParagraph || pk == ast.KindTextBlock
		}
	}
	return false
}

func (r *Renderer) writeSegments(w io.Writer, segments *text.Segments, indents bool) {
	for i, l := range segments.Sliced(0, segments.Len()) {
		if indents && i > 0 {
			_, _ = w.Write(r.indents)
		}
		_, _ = w.Write(l.Value(r.source))
	}
}

func (r *Renderer) writeContent(b []byte) {
	if len(r.indents) == 0 {
		_, _ = r.writer.Write(b)
	} else {
		reader := bufio.NewReader(bytes.NewReader(b))
		for i := 0; ; i++ {
			l, err := reader.ReadBytes('\n')
			if len(l) > 0 {
				if i > 0 {
					_, _ = r.writer.Write(r.indents)
				}
				_, _ = r.writer.Write(l)
			}
			if err != nil {
				break // EOF
			}
		}
	}
}

// modify link & image tags
func (r *Renderer) modifyHTMLTags(source []byte, target io.Writer) (bool, error) {
	modified := false
	z := html.NewTokenizer(bytes.NewReader(source))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return modified, nil // end of tokens
		}
		t := z.Token()
		if "a" == t.Data {
			for i, a := range t.Attr {
				if a.Key == "href" {
					dest, err := r.linkResolver(a.Val, false)
					if err != nil {
						return modified, err
					}
					if a.Val != dest {
						t.Attr[i].Val = dest
						modified = true
					}
					break
				}
			}
		} else if "img" == t.Data {
			for i, a := range t.Attr {
				if a.Key == "src" {
					dest, err := r.linkResolver(a.Val, true)
					if err != nil {
						return modified, err
					}
					if a.Val != dest {
						t.Attr[i].Val = dest
						modified = true
					}
					break
				}
			}
		}
		_, _ = target.Write([]byte(t.String()))
	}
}

func (r *Renderer) calcEmphasisChar(n ast.Node) (ch byte, txt []byte) {
	ch = '*' // default char
	// check if first emphasis child determines the char
	if n.Kind() == ast.KindEmphasis && n.FirstChild() != nil && n.FirstChild().Kind() == ast.KindEmphasis {
		if n.(*ast.Emphasis).Level == 1 && n.FirstChild().(*ast.Emphasis).Level == 1 {
			cch, _ := r.calcEmphasisChar(n.FirstChild())
			if cch == '*' {
				ch = '_' // handle nested <em> case
			}
			return
		}
	}
	// get node text
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindText:
			txt = append(txt, c.Text(r.source)...)
		case ast.KindEmphasis: // skip
		default:
			_, t := r.calcEmphasisChar(c)
			txt = append(txt, t...)
		}
	}
	// determine char
	if n.Kind() == ast.KindEmphasis {
		for i, b := range txt {
			if b == '*' {
				if i-1 >= 0 && txt[i-1] == '\\' {
					continue
				}
				ch = '_' // unescaped asterisk -> switch to underscore
				break
			}
		}
	}
	return
}

// if text could become different AST element additional indents must be added
func (r *Renderer) additionalIndents(text []byte, n *ast.Text) {
	// if there is a new line or intended new line before the text
	if n.PreviousSibling() != nil && len(text) > 0 {
		idx := bytes.LastIndexByte(r.writer.Bytes(), '\n')
		if idx == -1 {
			idx = 0
		}
		last := r.writer.Bytes()[idx:]
		if len(last) > 0 && (last[len(last)-1] == '\n' || bytes.Equal(last[1:], r.indents)) {
			var addIndents bool
			switch text[0] {
			case '-', '+', '*', '#', '=':
				addIndents = true
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				addIndents = marker.Match(text)
			}
			if addIndents {
				_, _ = r.writer.Write([]byte("    "))
			}
		}
	}
}

// sets blank previous line if needed
func breakBlockquoteLazyContinuation(n ast.Node) {
	if n != nil && !n.HasBlankPreviousLines() {
		if n.Kind() == ast.KindParagraph || n.Kind() == ast.KindTextBlock {
			n.SetBlankPreviousLines(true)
		}
	}
}

// build ListItem marker
func buildListMarker(n *ast.ListItem) []byte {
	p := n.Parent().(*ast.List)
	if p.IsOrdered() {
		return []byte(fmt.Sprintf("%d%c ", p.Start, p.Marker))
	}
	return []byte{p.Marker, ' '}
}

func isClassicAutolink(link []byte, cnt []byte) bool {
	if containsTrailingPunctuation(link[len(link)-1]) {
		return true
	}
	if len(cnt) > 0 && !isDelimiter(cnt[len(cnt)-1]) {
		return true
	}
	if !bytes.Equal(link, util.URLEscape(link, false)) {
		return true
	}
	if http.Match(link) {
		if len(cnt) > 0 && cnt[len(cnt)-1] == '(' {
			// workaround for using default Linkify URL regex in Hugo
			// `(https://foo.bar).` will be rendered as classic autolink `(<https://foo.bar>).`
			return true
		}
		return false // GFM autolink http/s
	}
	if www.Match(link) {
		return false // GFM autolink www
	}
	if email.Match(link) {
		return false // GFM autolink email
	}
	return true
}

func isDelimiter(b byte) bool {
	switch b {
	case '\n', '\r', '\t', '\f', '\v', '\x85', '\xa0', ' ', '*', '_', '~', '(':
		{
			return true
		}
	default:
		return false
	}
}

func containsTrailingPunctuation(b byte) bool {
	switch b {
	case '?', '!', '.', ',', ':', '*', '_', '~':
		{
			return true
		}
	default:
		return false
	}
}

func getLinkTitleWrapper(title []byte) byte {
	var dq, sq bool
	for i, b := range title {
		if b == '"' {
			if i-1 >= 0 && title[i-1] == '\\' {
				continue
			}
			dq = true
		} else if b == '\'' {
			if i-1 >= 0 && title[i-1] == '\\' {
				continue
			}
			sq = true
		}
	}
	if dq && sq {
		return '('
	} else if dq {
		return '\''
	}
	return '"'
}

func wrapLinkDestination(dest []byte) bool {
	var lp, elp int
	var rp, erp int
	for i, b := range dest {
		// check for controls & space
		if b <= '\x1f' || b == '\x7f' || b == '\x20' {
			return true
		}
		if b == '(' {
			lp++
			if i-1 >= 0 && dest[i-1] == '\\' {
				elp++
			}
		} else if b == ')' {
			rp++
			if i-1 >= 0 && dest[i-1] == '\\' {
				erp++
			}
		}
	}
	// check parentheses
	return (lp-elp)-(rp-elp) != 0
}

func nextIsLineBreak(next ast.Node, source []byte) bool {
	if next != nil && next.Kind() == ast.KindText {
		n := next.(*ast.Text)
		if n.HardLineBreak() || n.SoftLineBreak() {
			cnt := n.Text(source)
			cnt = bytes.TrimSpace(cnt)
			return len(cnt) == 0
		}
	}
	return false
}

// escape pipes in code span when table scope
func escapePipes(t []byte) []byte {
	var et []byte
	idx := 0
	for i, b := range t {
		if b == '|' {
			if i > 0 && t[i-1] == '\\' {
				continue
			}
			if i == 0 {
				et = append(et, '\\', '|')
			} else {
				et = append(et, t[idx:i]...)
				et = append(et, '\\', '|')
			}
			idx = i + 1
		}
	}
	if idx > 0 {
		et = append(et, t[idx:]...)
		return et
	}
	return t
}
