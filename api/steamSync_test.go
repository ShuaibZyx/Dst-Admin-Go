package api

import "testing"

func TestExtractSteamID64(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://steamcommunity.com/profiles/76561198000000000", "76561198000000000"},
		{"https://steamcommunity.com/profiles/76561198000000000/somepage", "76561198000000000"},
		{"http://steamcommunity.com/id/SomeVanity", ""},
	}
	for _, c := range cases {
		if got := extractSteamID64(c.url); got != c.want {
			t.Errorf("extractSteamID64(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestExtractWorkshopIDs(t *testing.T) {
	html := `
		<a href="https://steamcommunity.com/sharedfiles/filedetails/?id=1185229307">A</a>
		<div data-publishedfileid="2929437733"></div>
		<a href="https://steamcommunity.com/sharedfiles/filedetails/?id=3838383">B</a>
		<a href="https://steamcommunity.com/sharedfiles/filedetails/?id=1185229307">dup</a>
	`
	got := extractWorkshopIDs(html)
	want := map[string]bool{"1185229307": true, "2929437733": true, "3838383": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %d unique ids", got, len(want))
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("意外 id: %s", id)
		}
	}
}
