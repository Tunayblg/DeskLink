package config

import (
	"net/url"
	"strings"
)

// Brand is a well-known service a home screen tile can open. The phone and the
// dashboard draw a small icon for it instead of the first letter of its name
// (see web/static/brands.js); anything else keeps the letter.
type Brand struct {
	Name  string   // the key of its icon in brands.js
	Tint  string   // the tile color as "r,g,b", when none was chosen
	Hosts []string // the web hosts it lives on, matched with or without a "www." style prefix
}

var brands = []Brand{
	{Name: "netflix", Tint: "229,9,20", Hosts: []string{"netflix.com"}},
	{Name: "youtube", Tint: "255,40,40", Hosts: []string{"youtube.com", "youtu.be"}},
	{Name: "twitch", Tint: "145,70,255", Hosts: []string{"twitch.tv"}},
	{Name: "spotify", Tint: "30,215,96", Hosts: []string{"spotify.com"}},
	{Name: "prime", Tint: "0,168,225", Hosts: []string{"primevideo.com"}},
	{Name: "disney", Tint: "40,90,230", Hosts: []string{"disneyplus.com"}},
}

// BrandFor recognizes a shortcut target: a web address on a known host, or the
// spotify: link of the desktop app.
func BrandFor(target string) (Brand, bool) {
	t := strings.TrimSpace(target)
	if strings.HasPrefix(strings.ToLower(t), "spotify:") {
		return brands[3], true
	}
	if !IsWebAddress(t) {
		return Brand{}, false
	}
	u, err := url.Parse(t)
	if err != nil {
		return Brand{}, false
	}
	host := strings.ToLower(u.Hostname())
	for _, b := range brands {
		for _, h := range b.Hosts {
			if host == h || strings.HasSuffix(host, "."+h) {
				return b, true
			}
		}
	}
	return Brand{}, false
}
