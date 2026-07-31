package pattern

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func mustCompile(t *testing.T, pat string, smart bool) *Pattern {
	t.Helper()
	p, err := Compile(pat, smart)
	if err != nil {
		t.Fatalf("Compile(%q) failed: %v", pat, err)
	}
	return p
}

func TestSingleCapture(t *testing.T) {
	p := mustCompile(t, `<h1>{%}</h1>`, false)
	got := p.FindFirst(`<body><h1>Hello</h1></body>`)
	if !reflect.DeepEqual(got, []string{"Hello"}) {
		t.Errorf("got %#v", got)
	}
}

func TestSkipMacro(t *testing.T) {
	p := mustCompile(t, `<h2>News</h2>{*}<ul>{%}</ul>`, false)
	got := p.FindFirst(`<h2>News</h2><p>intro</p><ul><li>a</li></ul>`)
	if !reflect.DeepEqual(got, []string{"<li>a</li>"}) {
		t.Errorf("got %#v", got)
	}
}

func TestMultipleCapturesOrder(t *testing.T) {
	p := mustCompile(t, `<a href="{%}">{%}</a>`, false)
	got := p.FindFirst(`<a href="/x">Title</a>`)
	if !reflect.DeepEqual(got, []string{"/x", "Title"}) {
		t.Errorf("got %#v", got)
	}
}

func TestTrailingBareCaptureIsGreedy(t *testing.T) {
	// A bare {%} must grab the whole region (Feed43: single-item feeds).
	p := mustCompile(t, `{%}`, false)
	region := "line1\nline2\nline3"
	items := p.FindAll(region, 0)
	if len(items) != 1 || items[0][0] != region {
		t.Errorf("got %#v", items)
	}
}

func TestTrailingAnchoredCaptureIsGreedy(t *testing.T) {
	// Greedy trailing capture runs to the end, not to the first newline.
	p := mustCompile(t, `Start:{%}`, false)
	got := p.FindFirst("Start:a\nb\nc")
	if !reflect.DeepEqual(got, []string{"a\nb\nc"}) {
		t.Errorf("got %#v", got)
	}
}

func TestInteriorCaptureIsLazy(t *testing.T) {
	p := mustCompile(t, `<li>{%}</li>`, false)
	items := p.FindAll(`<li>a</li><li>b</li>`, 0)
	want := [][]string{{"a"}, {"b"}}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("got %#v want %#v", items, want)
	}
}

func TestMacrosCrossNewlines(t *testing.T) {
	p := mustCompile(t, `<li>{%}</li>`, false)
	items := p.FindAll("<li>a\nmulti\nline</li>", 0)
	if len(items) != 1 || items[0][0] != "a\nmulti\nline" {
		t.Errorf("got %#v", items)
	}
}

func TestTrailingSkipStaysLazy(t *testing.T) {
	// A greedy trailing {*} would swallow the rest of the region and
	// collapse three items into one.
	p := mustCompile(t, `<li><a href="{%}">{%}</a>{*}`, false)
	items := p.FindAll(`<li><a href="/1">A</a>x</li><li><a href="/2">B</a>y</li><li><a href="/3">C</a>z</li>`, 0)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %#v", len(items), items)
	}
	if items[2][0] != "/3" || items[2][1] != "C" {
		t.Errorf("last item = %#v", items[2])
	}
}

func TestTrailingWhitespaceDoesNotDemoteGreedyCapture(t *testing.T) {
	// A textarea appends a newline when the user presses Enter; that
	// invisible character must not change what the pattern means.
	p := mustCompile(t, "Start:{%}\n", false)
	got := p.FindFirst("Start:a\nb\nc")
	if !reflect.DeepEqual(got, []string{"a\nb\nc"}) {
		t.Errorf("got %#v", got)
	}
}

func TestBarePatternWithTrailingNewlineStillGrabsRegion(t *testing.T) {
	p := mustCompile(t, "{%}\n  ", false)
	region := "no newlines here"
	items := p.FindAll(region, 0)
	if len(items) != 1 || items[0][0] != region {
		t.Errorf("got %#v", items)
	}
}

func TestFindLast(t *testing.T) {
	page := `<li>1</li><li>2</li><li>3</li><li>4</li><li>5</li>`
	p := mustCompile(t, `<li>{%}</li>`, false)

	last2 := p.FindLast(page, 2)
	want := [][]string{{"4"}, {"5"}}
	if !reflect.DeepEqual(last2, want) {
		t.Errorf("FindLast(2) = %#v, want %#v", last2, want)
	}
	// Asking for more than exist returns everything, in order.
	all := p.FindLast(page, 99)
	if len(all) != 5 || all[0][0] != "1" || all[4][0] != "5" {
		t.Errorf("FindLast(99) = %#v", all)
	}
	if got := p.FindLast(page, 0); len(got) != 5 {
		t.Errorf("FindLast(0) should behave like FindAll, got %d", len(got))
	}
}

func TestFindAllStopsEarly(t *testing.T) {
	// The limit must bound the work, not just the result: a page with a
	// huge number of matches should still be cheap when only a few are
	// wanted. A pathological page makes an all-matches scan visibly slow.
	page := strings.Repeat("<li>x</li>", 200_000)
	p := mustCompile(t, `<li>{%}</li>`, false)
	start := time.Now()
	items := p.FindAll(page, 5)
	elapsed := time.Since(start)
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("limited FindAll took %v — limit is not short-circuiting the scan", elapsed)
	}
}

func TestFindAllLimit(t *testing.T) {
	p := mustCompile(t, `<li>{%}</li>`, false)
	items := p.FindAll(`<li>a</li><li>b</li><li>c</li>`, 2)
	if len(items) != 2 {
		t.Errorf("got %d items", len(items))
	}
}

func TestSmartWhitespace(t *testing.T) {
	p := mustCompile(t, "<ul>\n  <li>{%}</li>", true)
	got := p.FindFirst("<ul>\r\n\t\t<li>x</li>")
	if !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("got %#v", got)
	}
}

func TestStrictWhitespace(t *testing.T) {
	p := mustCompile(t, "<ul> <li>{%}</li>", false)
	if got := p.FindFirst("<ul>  <li>x</li>"); got != nil {
		t.Errorf("strict mode should not match differing whitespace, got %#v", got)
	}
}

func TestCRLFPageNormalization(t *testing.T) {
	// Pages arrive normalized via NormalizePage; pattern newlines are
	// normalized in Compile, so LF patterns match CRLF pages even in
	// strict mode.
	p := mustCompile(t, "a\nb{%}c", false)
	page := NormalizePage("a\r\nbXc")
	if got := p.FindFirst(page); !reflect.DeepEqual(got, []string{"X"}) {
		t.Errorf("got %#v", got)
	}
}

func TestRegexMetacharactersAreLiteral(t *testing.T) {
	p := mustCompile(t, `price ($) [USD]: {%}.`, false)
	got := p.FindFirst(`price ($) [USD]: 42.`)
	if !reflect.DeepEqual(got, []string{"42"}) {
		t.Errorf("got %#v", got)
	}
}

func TestLiteralBraces(t *testing.T) {
	p := mustCompile(t, `{"name": "{%}"}`, false)
	got := p.FindFirst(`{"name": "Ada"}`)
	if !reflect.DeepEqual(got, []string{"Ada"}) {
		t.Errorf("got %#v", got)
	}
}

func TestEmptyRegionYieldsNoItems(t *testing.T) {
	p := mustCompile(t, `{%}`, false)
	if items := p.FindAll("", 0); len(items) != 0 {
		t.Errorf("got %#v", items)
	}
}

func TestEmptyPatternRejected(t *testing.T) {
	if _, err := Compile("   ", true); err == nil {
		t.Error("expected error for blank pattern")
	}
}

func TestTooLongPatternRejected(t *testing.T) {
	if _, err := Compile(strings.Repeat("a", MaxPatternLen+1), true); err == nil {
		t.Error("expected error for oversized pattern")
	}
}

func TestNumCaptures(t *testing.T) {
	p := mustCompile(t, `{%}-{*}-{%}`, false)
	if p.NumCaptures() != 2 {
		t.Errorf("got %d", p.NumCaptures())
	}
}

func TestRender(t *testing.T) {
	caps := []string{"first", "second"}
	cases := []struct{ tmpl, want string }{
		{"{%1}", "first"},
		{"{%2} / {%1}", "second / first"},
		{"{%3}", ""},        // out of range → empty
		{"{%0}", ""},        // invalid index → empty
		{"literal", "literal"},
		{"", ""},
		{"a {%1} b {%2} c", "a first b second c"},
	}
	for _, c := range cases {
		if got := Render(c.tmpl, caps); got != c.want {
			t.Errorf("Render(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
}

func TestRenderKeepsUnknownBraces(t *testing.T) {
	if got := Render("{%x} {} {%}", []string{"v"}); got != "{%x} {} {%}" {
		t.Errorf("got %q", got)
	}
}

func TestRealWorldShape(t *testing.T) {
	page := `<html><body>
<h2>Belarus News</h2>
<p>intro text</p>
<ul class="news">
  <li><a href="/news/1">First headline</a><span>2026-07-30</span></li>
  <li><a href="/news/2">Second headline</a><span>2026-07-31</span></li>
</ul>
</body></html>`
	g := mustCompile(t, `<ul class="news">{%}</ul>`, true)
	caps := g.FindFirst(NormalizePage(page))
	if caps == nil {
		t.Fatal("global pattern did not match")
	}
	it := mustCompile(t, `<li><a href="{%}">{%}</a><span>{%}</span></li>`, true)
	items := it.FindAll(caps[0], 0)
	if len(items) != 2 {
		t.Fatalf("got %d items: %#v", len(items), items)
	}
	if items[1][1] != "Second headline" || items[1][0] != "/news/2" {
		t.Errorf("got %#v", items[1])
	}
	title := Render("{%2} ({%3})", items[0])
	if title != "First headline (2026-07-30)" {
		t.Errorf("got %q", title)
	}
}
