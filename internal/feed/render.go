// Package feed renders generated items as RSS 2.0 and JSON Feed 1.1.
package feed

import (
	"encoding/json"
	"encoding/xml"
	"time"
)

// Item is one output feed entry.
type Item struct {
	Title   string
	Link    string
	Content string // HTML allowed; escaped on output
	GUID    string
	PubDate time.Time // zero = omit
}

// Meta describes the feed itself.
type Meta struct {
	Title       string
	Link        string
	Description string
	SelfURL     string // absolute URL of the feed document itself
	LastBuild   time.Time
}

type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	AtomNS  string     `xml:"xmlns:atom,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Generator     string    `xml:"generator"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	AtomLink      *atomLink `xml:"atom:link,omitempty"`
	Items         []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link,omitempty"`
	Description string  `xml:"description,omitempty"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate,omitempty"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// RSS renders an RSS 2.0 document.
func RSS(meta Meta, items []Item) ([]byte, error) {
	ch := rssChannel{
		Title:       meta.Title,
		Link:        meta.Link,
		Description: meta.Description,
		Generator:   "FeedForge",
	}
	if !meta.LastBuild.IsZero() {
		ch.LastBuildDate = meta.LastBuild.UTC().Format(time.RFC1123Z)
	}
	if meta.SelfURL != "" {
		ch.AtomLink = &atomLink{Href: meta.SelfURL, Rel: "self", Type: "application/rss+xml"}
	}
	for _, it := range items {
		ri := rssItem{
			Title:       sanitizeXML(it.Title),
			Link:        sanitizeXML(it.Link),
			Description: sanitizeXML(it.Content),
			GUID:        rssGUID{IsPermaLink: "false", Value: sanitizeXML(it.GUID)},
		}
		if !it.PubDate.IsZero() {
			ri.PubDate = it.PubDate.UTC().Format(time.RFC1123Z)
		}
		ch.Items = append(ch.Items, ri)
	}
	doc := rssDoc{
		Version: "2.0",
		AtomNS:  "http://www.w3.org/2005/Atom",
		Channel: ch,
	}
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

type jsonFeed struct {
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	HomePageURL string         `json:"home_page_url,omitempty"`
	FeedURL     string         `json:"feed_url,omitempty"`
	Description string         `json:"description,omitempty"`
	Items       []jsonFeedItem `json:"items"`
}

type jsonFeedItem struct {
	ID            string `json:"id"`
	URL           string `json:"url,omitempty"`
	Title         string `json:"title,omitempty"`
	ContentHTML   string `json:"content_html,omitempty"`
	DatePublished string `json:"date_published,omitempty"`
}

// JSONFeed renders a JSON Feed 1.1 document.
func JSONFeed(meta Meta, items []Item) ([]byte, error) {
	jf := jsonFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       meta.Title,
		HomePageURL: meta.Link,
		FeedURL:     meta.SelfURL,
		Description: meta.Description,
		Items:       []jsonFeedItem{},
	}
	for _, it := range items {
		ji := jsonFeedItem{
			ID:          it.GUID,
			URL:         it.Link,
			Title:       it.Title,
			ContentHTML: it.Content,
		}
		if !it.PubDate.IsZero() {
			ji.DatePublished = it.PubDate.UTC().Format(time.RFC3339)
		}
		jf.Items = append(jf.Items, ji)
	}
	return json.MarshalIndent(jf, "", "  ")
}

// sanitizeXML strips control characters that are illegal in XML 1.0 —
// scraped pages occasionally contain them, and encoding/xml would emit an
// unparseable document otherwise.
func sanitizeXML(s string) string {
	clean := make([]rune, 0, len(s))
	for _, r := range s {
		if r == 0x9 || r == 0xA || r == 0xD || (r >= 0x20 && r != 0xFFFE && r != 0xFFFF) {
			clean = append(clean, r)
		}
	}
	return string(clean)
}
