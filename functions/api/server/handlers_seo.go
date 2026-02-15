package server

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"github.com/raymonstah/asianamericanswiki/internal/humandao"
)

func (s *ServerHTML) HandlerRobots(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/plain")
	if _, err := fmt.Fprintf(w, `User-agent: *
Allow: /
Sitemap: https://asianamericans.wiki/sitemap.xml
`); err != nil {
		return err
	}
	return nil
}

type URL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float32 `xml:"priority,omitempty"`
}

type URLSet struct {
	XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []URL    `xml:"url"`
}

func (s *ServerHTML) HandlerSitemap(w http.ResponseWriter, r *http.Request) error {
	s.lock.Lock()
	humans := make([]humandao.Human, len(s.humans))
	copy(humans, s.humans)
	s.lock.Unlock()

	var urls []URL
	// Add home page
	urls = append(urls, URL{
		Loc:        "https://asianamericans.wiki/",
		LastMod:    time.Now().Format("2006-01-02"),
		ChangeFreq: "daily",
		Priority:   1.0,
	})

	// Add about page
	urls = append(urls, URL{
		Loc:        "https://asianamericans.wiki/about",
		Priority:   0.5,
	})

	// Add humans list
	urls = append(urls, URL{
		Loc:        "https://asianamericans.wiki/humans",
		ChangeFreq: "daily",
		Priority:   0.8,
	})

	// Add individual humans
	for _, human := range humans {
		if human.Draft {
			continue
		}
		lastMod := human.UpdatedAt
		if lastMod.IsZero() {
			lastMod = human.CreatedAt
		}
		if lastMod.IsZero() {
			lastMod = time.Now()
		}

		urls = append(urls, URL{
			Loc:      fmt.Sprintf("https://asianamericans.wiki/humans/%s", human.Path),
			LastMod:  lastMod.Format("2006-01-02"),
			Priority: 0.7,
		})
	}

	urlSet := URLSet{URLs: urls}
	w.Header().Set("Content-Type", "application/xml")
	if _, err := fmt.Fprintf(w, "%s", xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	return encoder.Encode(urlSet)
}
