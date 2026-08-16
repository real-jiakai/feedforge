package server

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	"unicode/utf8"

	"github.com/real-jiakai/feedforge/internal/fetch"
	"github.com/real-jiakai/feedforge/internal/store"
)

const samplePage = `<html><body>
<h2>Test News</h2>
<ul class="news">
  <li><a href="/articles/1">First &amp; foremost</a><p>Body <b>one</b></p></li>
  <li><a href="/articles/2">Second <em>story</em></a><p>Body two</p></li>
  <li><a href="https://other.example/x">Third</a><p>Body &lt;3</p></li>
</ul>
</body></html>`

func newTestServer(t *testing.T) (*httptest.Server, *store.Store, *httptest.Server) {
	t.Helper()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, samplePage)
	}))
	t.Cleanup(source.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Config{
		Store: st,
		// allowPrivate=true: the test source server listens on loopback.
		Fetcher: fetch.New(true, 5*1024*1024, 10*time.Second),
		WebFS:   fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>ui</html>")}},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	app := httptest.NewServer(handler)
	t.Cleanup(app.Close)
	return app, st, source
}

// newClient returns an HTTP client with a cookie jar, so it can hold a
// session across requests.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

// register creates an account through the API and leaves the session cookie
// in the client's jar. The first account on a server becomes the admin.
func register(t *testing.T, app *httptest.Server, c *http.Client, username string) userInfo {
	t.Helper()
	resp := postJSON(t, c, app.URL+"/api/auth/register",
		map[string]any{"username": username, "password": "password123"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("register %q: HTTP %d: %s", username, resp.StatusCode, raw)
	}
	var u userInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	return u
}

// adminClient registers the server's first account and returns its client.
func adminClient(t *testing.T, app *httptest.Server) *http.Client {
	t.Helper()
	c := newClient(t)
	register(t, app, c, "admin")
	return c
}

func jsonReq(t *testing.T, c *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func postJSON(t *testing.T, c *http.Client, url string, body any) *http.Response {
	t.Helper()
	return jsonReq(t, c, http.MethodPost, url, body)
}

func mkFeed(sourceURL string) map[string]any {
	return map[string]any{
		"title":           "Test Feed",
		"sourceUrl":       sourceURL,
		"globalPattern":   `<ul class="news">{%}</ul>`,
		"itemPattern":     `<li><a href="{%}">{%}</a><p>{%}</p></li>`,
		"itemTitle":       "{%2}",
		"itemLink":        "{%1}",
		"itemContent":     "{%3}",
		"smartWhitespace": true,
		"maxItems":        25,
		"ttlMinutes":      30,
	}
}

func createFeed(t *testing.T, c *http.Client, app *httptest.Server, source *httptest.Server) store.Feed {
	t.Helper()
	resp := postJSON(t, c, app.URL+"/api/feeds", mkFeed(source.URL))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("create feed: HTTP %d: %s", resp.StatusCode, raw)
	}
	var f store.Feed
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		t.Fatal(err)
	}
	return f
}

func listFeeds(t *testing.T, c *http.Client, app *httptest.Server) []store.Feed {
	t.Helper()
	resp, err := c.Get(app.URL + "/api/feeds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list feeds: HTTP %d", resp.StatusCode)
	}
	var feeds []store.Feed
	if err := json.NewDecoder(resp.Body).Decode(&feeds); err != nil {
		t.Fatal(err)
	}
	return feeds
}

type rssOut struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			Desc    string `xml:"description"`
			GUID    string `xml:"guid"`
			PubDate string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func fetchRSS(t *testing.T, url string) (rssOut, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: HTTP %d: %s", url, resp.StatusCode, raw)
	}
	var out rssOut
	if err := xml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("invalid RSS XML: %v\n%s", err, raw)
	}
	return out, raw
}

// ---------- feed pipeline ----------

func TestEndToEndRSS(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)

	out, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if out.Channel.Title != "Test Feed" {
		t.Errorf("channel title = %q", out.Channel.Title)
	}
	if len(out.Channel.Items) != 3 {
		t.Fatalf("got %d items:\n%s", len(out.Channel.Items), raw)
	}
	// Entities decoded in titles, tags stripped.
	if out.Channel.Items[0].Title != "First & foremost" {
		t.Errorf("item0 title = %q", out.Channel.Items[0].Title)
	}
	if out.Channel.Items[1].Title != "Second story" {
		t.Errorf("item1 title = %q", out.Channel.Items[1].Title)
	}
	// Relative links resolved against the source URL.
	if want := source.URL + "/articles/1"; out.Channel.Items[0].Link != want {
		t.Errorf("item0 link = %q, want %q", out.Channel.Items[0].Link, want)
	}
	if out.Channel.Items[2].Link != "https://other.example/x" {
		t.Errorf("item2 link = %q", out.Channel.Items[2].Link)
	}
	// Content keeps its HTML (escaped in transit, unescaped after parsing).
	if out.Channel.Items[0].Desc != "Body <b>one</b>" {
		t.Errorf("item0 desc = %q", out.Channel.Items[0].Desc)
	}
	if out.Channel.Items[0].PubDate == "" {
		t.Error("item0 has no pubDate")
	}
}

func TestGUIDAndPubDateStableAcrossRebuilds(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)

	first, _ := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	// Force a refetch (cache bypass) and compare.
	resp := postJSON(t, c, app.URL+"/api/feeds/"+f.ID+"/refresh", map[string]any{})
	resp.Body.Close()
	second, _ := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")

	if first.Channel.Items[0].GUID != second.Channel.Items[0].GUID {
		t.Error("GUID changed across rebuilds")
	}
	if first.Channel.Items[0].PubDate != second.Channel.Items[0].PubDate {
		t.Error("pubDate changed across rebuilds")
	}
}

func TestJSONFeedOutput(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)

	resp, err := http.Get(app.URL + "/feeds/" + f.ID + ".json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var jf struct {
		Version string `json:"version"`
		Title   string `json:"title"`
		Items   []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			ContentHTML string `json:"content_html"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jf); err != nil {
		t.Fatal(err)
	}
	if jf.Version != "https://jsonfeed.org/version/1.1" || len(jf.Items) != 3 {
		t.Errorf("unexpected JSON feed: %+v", jf)
	}
	if jf.Items[0].ContentHTML != "Body <b>one</b>" {
		t.Errorf("content_html = %q", jf.Items[0].ContentHTML)
	}
}

func TestSSRFGuardBlocksLoopback(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, samplePage)
	}))
	defer source.Close()

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Config{
		Store:   st,
		Fetcher: fetch.New(false, 5*1024*1024, 10*time.Second), // guard ON
		WebFS:   fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ui")}},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	app := httptest.NewServer(handler)
	defer app.Close()

	c := adminClient(t, app)
	f := createFeed(t, c, app, source)
	resp, err := http.Get(app.URL + "/feeds/" + f.ID + ".xml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 for loopback source, got %d", resp.StatusCode)
	}
}

func TestPreviewEndpoint(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	body := map[string]any{
		"sourceUrl":       source.URL,
		"globalPattern":   `<ul class="news">{%}</ul>`,
		"itemPattern":     `<li><a href="{%}">{%}</a><p>{%}</p></li>`,
		"itemTitle":       "{%2}",
		"itemLink":        "{%1}",
		"smartWhitespace": true,
		"includePage":     true,
	}
	resp := postJSON(t, c, app.URL+"/api/preview", body)
	defer resp.Body.Close()
	var pv previewResponse
	if err := json.NewDecoder(resp.Body).Decode(&pv); err != nil {
		t.Fatal(err)
	}
	if pv.FetchError != "" || pv.GlobalError != "" || pv.ItemError != "" {
		t.Fatalf("unexpected errors: %+v", pv)
	}
	if pv.TotalMatches != 3 || len(pv.Items) != 3 {
		t.Fatalf("expected 3 items, got %+v", pv)
	}
	if pv.Items[0].Title != "First & foremost" {
		t.Errorf("preview title = %q", pv.Items[0].Title)
	}
	if pv.PageExcerpt == "" {
		t.Error("expected page excerpt when includePage=true")
	}
}

func TestXMLEscapingOfHostileContent(t *testing.T) {
	hostile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<ul class="news"><li><a href="/a?x=1&y=2">]]></title><script>alert(1)</script></a><p>&"'<&</p></li></ul>`)
	}))
	defer hostile.Close()

	app, _, _ := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, hostile)

	out, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if len(out.Channel.Items) != 1 {
		t.Fatalf("got %d items:\n%s", len(out.Channel.Items), raw)
	}
	// The document must stay well-formed (xml.Unmarshal above already
	// proves it) and must not contain a raw <script> tag.
	if strings.Contains(string(raw), "<script>") {
		t.Error("raw script tag leaked into XML output")
	}
}

func TestDemoPageServedInProcess(t *testing.T) {
	// Even with the SSRF guard ON, the server's own /demo page works.
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Config{
		Store:   st,
		Fetcher: fetch.New(false, 5*1024*1024, 10*time.Second),
		WebFS:   fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ui")}},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	app := httptest.NewServer(handler)
	defer app.Close()

	c := adminClient(t, app)
	feedDef := map[string]any{
		"title":           "Demo",
		"sourceUrl":       store.DemoSourceURL,
		"globalPattern":   `<ul class="articles">{%}</ul>`,
		"itemPattern":     `<li class="article">{*}<a href="{%}">{%}</a>{*}<span class="date">{%}</span>`,
		"itemTitle":       "{%2}",
		"itemLink":        "{%1}",
		"smartWhitespace": true,
	}
	resp := postJSON(t, c, app.URL+"/api/feeds", feedDef)
	var f store.Feed
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	out, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if len(out.Channel.Items) != 6 {
		t.Fatalf("expected 6 demo items, got %d:\n%s", len(out.Channel.Items), raw)
	}
	// The demo's hrefs are relative and its source URL is a non-http
	// sentinel, so they cannot be resolved to an absolute http(s) URL and
	// are dropped rather than emitted as garbage.
	if out.Channel.Items[0].Title == "" {
		t.Errorf("demo item has no title: %#v", out.Channel.Items[0])
	}
}

func TestDemoSourceCannotBeSpoofedViaHostHeader(t *testing.T) {
	// A feed pointing at a real host must never be short-circuited to the
	// built-in demo HTML just because a client claims that Host.
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	def := mkFeed(source.URL + "/demo")
	resp := postJSON(t, c, app.URL+"/api/feeds", def)
	var f store.Feed
	_ = json.NewDecoder(resp.Body).Decode(&f)
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodGet, app.URL+"/feeds/"+f.ID+".xml", nil)
	req.Host = mustHost(t, source.URL)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	raw, _ := io.ReadAll(r.Body)
	if strings.Contains(string(raw), "FeedForge Demo News") {
		t.Error("demo HTML was substituted for a real source page via the Host header")
	}
}

func mustHost(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestCacheServesWithoutRefetch(t *testing.T) {
	hits := 0
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, samplePage)
	}))
	defer source.Close()

	app, _, _ := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)

	fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if hits != 1 {
		t.Errorf("source fetched %d times; want 1 (TTL cache)", hits)
	}
}

func TestHostileSchemesAreStrippedFromLinks(t *testing.T) {
	hostile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<ul class="news">`+
			`<li><a href="javascript:alert(1)">JS</a><p>a</p></li>`+
			`<li><a href="data:text/html,<script>alert(1)</script>">Data</a><p>b</p></li>`+
			`<li><a href="java&#115;cript:alert(2)">Entity</a><p>c</p></li>`+
			`<li><a href="/ok">Fine</a><p>d</p></li>`+
			`</ul>`)
	}))
	defer hostile.Close()

	app, _, _ := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, hostile)
	out, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")

	if len(out.Channel.Items) != 4 {
		t.Fatalf("got %d items:\n%s", len(out.Channel.Items), raw)
	}
	for i, it := range out.Channel.Items[:3] {
		if it.Link != "" {
			t.Errorf("item %d kept a non-http link: %q", i, it.Link)
		}
	}
	if !strings.HasSuffix(out.Channel.Items[3].Link, "/ok") {
		t.Errorf("legitimate relative link was dropped: %q", out.Channel.Items[3].Link)
	}
	if strings.Contains(strings.ToLower(string(raw)), "javascript:") {
		t.Error("javascript: URL reached the feed document")
	}
}

func TestReverseKeepsNewestWhenPageExceedsMaxItems(t *testing.T) {
	// An oldest-first page: new entries are appended at the bottom. With
	// reverse=true and maxItems=3 the feed must carry entries 8,7,6 —
	// taking the FIRST three matches would freeze it on 1,2,3 forever.
	var sb strings.Builder
	sb.WriteString(`<ul class="news">`)
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&sb, `<li><a href="/p%d">Post %d</a><p>body %d</p></li>`, i, i, i)
	}
	sb.WriteString(`</ul>`)
	page := sb.String()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, page)
	}))
	defer source.Close()

	app, _, _ := newTestServer(t)
	c := adminClient(t, app)
	def := mkFeed(source.URL)
	def["reverse"] = true
	def["maxItems"] = 3
	resp := postJSON(t, c, app.URL+"/api/feeds", def)
	var f store.Feed
	_ = json.NewDecoder(resp.Body).Decode(&f)
	resp.Body.Close()

	out, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if len(out.Channel.Items) != 3 {
		t.Fatalf("got %d items:\n%s", len(out.Channel.Items), raw)
	}
	want := []string{"Post 8", "Post 7", "Post 6"}
	for i, w := range want {
		if out.Channel.Items[i].Title != w {
			t.Errorf("item %d = %q, want %q", i, out.Channel.Items[i].Title, w)
		}
	}
}

func TestForcedRefreshReportsFailure(t *testing.T) {
	healthy := true
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, samplePage)
	}))
	defer source.Close()

	app, _, _ := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)
	fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml") // prime the cache

	healthy = false
	resp := postJSON(t, c, app.URL+"/api/feeds/"+f.ID+"/refresh", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("forced refresh of a broken source returned %d (%s); want 502",
			resp.StatusCode, body)
	}
}

func TestEditingFeedKeepsStatus(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)
	fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml") // produces a status

	def := mkFeed(source.URL)
	def["title"] = "Renamed"
	r := jsonReq(t, c, http.MethodPut, app.URL+"/api/feeds/"+f.ID, def)
	r.Body.Close()

	got, err := c.Get(app.URL + "/api/feeds/" + f.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	var after store.Feed
	if err := json.NewDecoder(got.Body).Decode(&after); err != nil {
		t.Fatal(err)
	}
	if after.Title != "Renamed" {
		t.Errorf("edit did not apply: %q", after.Title)
	}
	if after.LastFetchAt.IsZero() || after.LastItemCount != 3 {
		t.Errorf("editing wiped the feed status: fetchedAt=%v count=%d",
			after.LastFetchAt, after.LastItemCount)
	}
}

func TestSmartWhitespaceDefaultsOnForAPIClients(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Indented markup: only smart-whitespace matching finds it with a
		// single-space pattern.
		_, _ = io.WriteString(w, "<ul class=\"news\">\n\t<li><a href=\"/a\">A</a>\n\t\t<p>body</p></li>\n</ul>")
	}))
	defer source.Close()

	app, _, _ := newTestServer(t)
	c := adminClient(t, app)
	def := map[string]any{
		"title":         "No flag",
		"sourceUrl":     source.URL,
		"globalPattern": `<ul class="news">{%}</ul>`,
		"itemPattern":   `<li><a href="{%}">{%}</a> <p>{%}</p></li>`,
		"itemTitle":     "{%2}",
		"itemLink":      "{%1}",
		// smartWhitespace deliberately omitted
	}
	resp := postJSON(t, c, app.URL+"/api/feeds", def)
	var f store.Feed
	_ = json.NewDecoder(resp.Body).Decode(&f)
	resp.Body.Close()
	if !f.SmartWhitespace {
		t.Fatal("smartWhitespace should default to true when omitted")
	}
	out, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if len(out.Channel.Items) != 1 {
		t.Fatalf("got %d items:\n%s", len(out.Channel.Items), raw)
	}
}

func TestSelfLinkFollowsRequestNotCache(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)

	// Prime the cache through a request claiming an attacker-chosen Host.
	req, _ := http.NewRequest(http.MethodGet, app.URL+"/feeds/"+f.ID+".xml", nil)
	req.Host = "evil.example"
	r1, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r1.Body.Close()

	// A normal request must not be served the poisoned self-link.
	_, raw := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if strings.Contains(string(raw), "evil.example") {
		t.Error("cached feed carried another client's Host header in atom:link")
	}
}

func TestInvalidIDRejected(t *testing.T) {
	app, _, _ := newTestServer(t)
	for _, path := range []string{
		"/feeds/UPPER.xml", "/feeds/..%2Fx.xml", "/feeds/abc.exe", "/feeds/ab.xml",
	} {
		resp, err := http.Get(app.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestCutStringKeepsValidUTF8(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"}, // shorter than max: untouched
		{"hello", 4, "hell"},   // plain ASCII cut
		{"aé-more", 3, "aé"},   // cut lands right after a complete rune
		{"aé-more", 2, "a"},    // cut lands inside a rune
		{"标题很长", 7, "标题"},      // CJK: 7 bytes = 2 runes + 1 partial byte
	}
	for _, c := range cases {
		got, _ := cutString(c.in, c.max)
		if got != c.want {
			t.Errorf("cutString(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("cutString(%q, %d) produced invalid UTF-8: %q", c.in, c.max, got)
		}
	}
}

// ---------- accounts ----------

func TestLoginRequiredForAPI(t *testing.T) {
	app, _, source := newTestServer(t)

	// Anonymous API access is refused across the board.
	anon := newClient(t)
	resp, err := anon.Get(app.URL + "/api/feeds")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous list: got %d, want 401", resp.StatusCode)
	}
	for _, path := range []string{"/api/feeds", "/api/preview"} {
		r := postJSON(t, anon, app.URL+path, mkFeed(source.URL))
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous POST %s: got %d, want 401", path, r.StatusCode)
		}
	}

	// Feed output stays public: readers cannot sign in.
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)
	out, _ := fetchRSS(t, app.URL+"/feeds/"+f.ID+".xml")
	if len(out.Channel.Items) == 0 {
		t.Error("feed output should be public")
	}
}

func TestFirstUserIsAdminAndGetsSeededRecipes(t *testing.T) {
	app, _, _ := newTestServer(t)
	c := newClient(t)
	u := register(t, app, c, "alice")
	if !u.IsAdmin {
		t.Error("first registered user should be the admin")
	}
	// A fresh instance seeds the two built-in recipe feeds, so "Your
	// feeds" starts as exactly the feeds this instance is meant to serve.
	feeds := listFeeds(t, c, app)
	if len(feeds) != len(Recipes) {
		t.Fatalf("got %d seeded feeds, want %d", len(feeds), len(Recipes))
	}
	seen := map[string]bool{}
	for _, f := range feeds {
		seen[f.SourceURL] = true
	}
	for _, rc := range Recipes {
		if !seen[rc.Feed.SourceURL] {
			t.Errorf("recipe %q was not seeded", rc.ID)
		}
	}
}

func TestRegistrationDisabledByDefault(t *testing.T) {
	app, _, _ := newTestServer(t)
	admin := adminClient(t, app)

	// Second registration is refused while the toggle is off.
	bob := newClient(t)
	resp := postJSON(t, bob, app.URL+"/api/auth/register",
		map[string]any{"username": "bob", "password": "password123"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("second registration: got %d, want 403", resp.StatusCode)
	}

	// The admin enables registration; now it works, and the new account
	// is a regular user.
	r := jsonReq(t, admin, http.MethodPut, app.URL+"/api/admin/settings",
		map[string]any{"registrationEnabled": true})
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("enable registration: got %d", r.StatusCode)
	}
	u := register(t, app, bob, "bob")
	if u.IsAdmin {
		t.Error("second user must not be an admin")
	}

	// Regular users cannot touch the admin settings.
	r2 := jsonReq(t, bob, http.MethodPut, app.URL+"/api/admin/settings",
		map[string]any{"registrationEnabled": false})
	r2.Body.Close()
	if r2.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin toggling registration: got %d, want 403", r2.StatusCode)
	}
}

func TestUsersCannotTouchEachOthersFeeds(t *testing.T) {
	app, _, source := newTestServer(t)
	admin := adminClient(t, app)
	r := jsonReq(t, admin, http.MethodPut, app.URL+"/api/admin/settings",
		map[string]any{"registrationEnabled": true})
	r.Body.Close()
	bob := newClient(t)
	register(t, app, bob, "bob")

	f := createFeed(t, admin, app, source)

	// Bob's list shows only his own (zero) feeds.
	if feeds := listFeeds(t, bob, app); len(feeds) != 0 {
		t.Errorf("bob sees %d feeds that are not his", len(feeds))
	}
	// Read, update, delete and refresh of someone else's feed all 404.
	resp, err := bob.Get(app.URL + "/api/feeds/" + f.ID)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("bob reading admin's feed: got %d, want 404", resp.StatusCode)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodPut, "/api/feeds/" + f.ID},
		{http.MethodDelete, "/api/feeds/" + f.ID},
		{http.MethodPost, "/api/feeds/" + f.ID + "/refresh"},
	} {
		r := jsonReq(t, bob, tc.method, app.URL+tc.path, mkFeed(source.URL))
		r.Body.Close()
		if r.StatusCode != http.StatusNotFound {
			t.Errorf("bob %s %s: got %d, want 404", tc.method, tc.path, r.StatusCode)
		}
	}
	// The feed is untouched for its owner.
	resp2, err := admin.Get(app.URL + "/api/feeds/" + f.ID)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("owner lost access to their feed: %d", resp2.StatusCode)
	}
}

func TestLegacyFeedsAdoptedAndMissingRecipesSeeded(t *testing.T) {
	app, st, source := newTestServer(t)

	// A feed from a pre-account data directory has no owner.
	legacy := &store.Feed{
		Title:       "Legacy",
		SourceURL:   source.URL,
		ItemPattern: `<li><a href="{%}">{%}</a>`,
	}
	if err := st.Create(legacy); err != nil {
		t.Fatal(err)
	}

	c := newClient(t)
	register(t, app, c, "admin")
	feeds := listFeeds(t, c, app)
	// The legacy feed is adopted AND the recipe feeds are still seeded:
	// carrying over an old data directory must not leave the admin without
	// the feeds this instance is meant to provide.
	if len(feeds) != 1+len(Recipes) {
		t.Fatalf("got %d feeds, want %d (legacy + recipes): %+v", len(feeds), 1+len(Recipes), feeds)
	}
	found := false
	for _, f := range feeds {
		if f.ID == legacy.ID {
			found = true
		}
	}
	if !found {
		t.Error("legacy feed was not adopted")
	}
}

func TestSeedingSkipsSourcesTheAdminAlreadyHas(t *testing.T) {
	app, st, _ := newTestServer(t)

	// A legacy feed already points at one of the recipe sources; seeding
	// must not create a duplicate for it.
	legacy := &store.Feed{
		Title:       "My own OSSInsight feed",
		SourceURL:   Recipes[0].Feed.SourceURL,
		ItemPattern: `<li><a href="{%}">{%}</a>`,
	}
	if err := st.Create(legacy); err != nil {
		t.Fatal(err)
	}

	c := newClient(t)
	register(t, app, c, "admin")
	feeds := listFeeds(t, c, app)
	if len(feeds) != len(Recipes) {
		t.Fatalf("got %d feeds, want %d (adopted feed + remaining recipes): %+v",
			len(feeds), len(Recipes), feeds)
	}
	bySource := map[string]int{}
	for _, f := range feeds {
		bySource[f.SourceURL]++
	}
	for src, n := range bySource {
		if n > 1 {
			t.Errorf("source %q appears %d times — seeding duplicated an existing feed", src, n)
		}
	}
}

func TestLogoutEndsSession(t *testing.T) {
	app, _, _ := newTestServer(t)
	c := adminClient(t, app)

	resp, err := c.Get(app.URL + "/api/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me while signed in: got %d", resp.StatusCode)
	}

	r := postJSON(t, c, app.URL+"/api/auth/logout", map[string]any{})
	r.Body.Close()

	resp2, err := c.Get(app.URL + "/api/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("me after logout: got %d, want 401", resp2.StatusCode)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	app, _, _ := newTestServer(t)
	adminClient(t, app) // creates user "admin" / "password123"

	for _, body := range []map[string]any{
		{"username": "admin", "password": "wrong-password"},
		{"username": "nobody", "password": "password123"},
	} {
		c := newClient(t)
		resp := postJSON(t, c, app.URL+"/api/auth/login", body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("login %v: got %d, want 401", body, resp.StatusCode)
		}
	}

	// The right credentials still work.
	c := newClient(t)
	resp := postJSON(t, c, app.URL+"/api/auth/login",
		map[string]any{"username": "admin", "password": "password123"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid login: got %d, want 200", resp.StatusCode)
	}
}

func TestConfigReflectsSetupState(t *testing.T) {
	app, _, _ := newTestServer(t)

	getConfig := func() map[string]any {
		t.Helper()
		resp, err := http.Get(app.URL + "/api/config")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var cfg map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
			t.Fatal(err)
		}
		return cfg
	}

	if cfg := getConfig(); cfg["needsSetup"] != true {
		t.Errorf("fresh instance should report needsSetup=true, got %v", cfg)
	}
	adminClient(t, app)
	if cfg := getConfig(); cfg["needsSetup"] != false {
		t.Errorf("after the first registration needsSetup should be false, got %v", cfg)
	}
}

func TestMutatingEndpointsRequireJSONContentType(t *testing.T) {
	app, _, source := newTestServer(t)
	c := adminClient(t, app)
	f := createFeed(t, c, app, source)

	raw, _ := json.Marshal(mkFeed(source.URL))
	// A cross-origin HTML form can only send these content types; they must
	// be refused so cookie-holding browsers can't be driven from any web
	// page. /refresh matters as much as the CRUD calls: without the check
	// any page could force refetches past the TTL.
	for _, tc := range []struct {
		path  string
		body  string
		ctype string
	}{
		{"/api/feeds", string(raw), "text/plain"},
		{"/api/feeds", string(raw), "application/x-www-form-urlencoded"},
		{"/api/feeds/" + f.ID + "/refresh", "", "text/plain"},
		{"/api/feeds/" + f.ID + "/refresh", "", ""},
		{"/api/auth/register", `{"username":"x","password":"password123"}`, "text/plain"},
		{"/api/auth/login", `{"username":"admin","password":"password123"}`, "text/plain"},
		{"/api/auth/logout", "", "text/plain"},
	} {
		req, _ := http.NewRequest(http.MethodPost, app.URL+tc.path, strings.NewReader(tc.body))
		if tc.ctype != "" {
			req.Header.Set("Content-Type", tc.ctype)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("POST %s (%s): got %d, want 415", tc.path, tc.ctype, resp.StatusCode)
		}
	}
}
