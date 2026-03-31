package editor

import (
	"strings"
	"testing"

	"github.com/clintecker/muxwarp/internal/sshconfig"
)

// testHostsComplete returns a sample set of SSH hosts for testing autocomplete.
func testHostsComplete() []sshconfig.Host {
	return []sshconfig.Host{
		{Alias: "atlas", HostName: "192.168.1.50", User: "alice", Port: "22"},
		{Alias: "forge", HostName: "forge.example.com", User: "deploy", Port: "2222"},
		{Alias: "bastion", HostName: "10.0.0.1", User: "admin", Port: "22"},
	}
}

func assertFilteredContains(t *testing.T, filtered []sshconfig.Host, alias string) {
	t.Helper()
	for _, h := range filtered {
		if h.Alias == alias {
			return
		}
	}
	t.Errorf("expected filtered hosts to contain %q", alias)
}

func assertFilteredExcludes(t *testing.T, filtered []sshconfig.Host, alias string) {
	t.Helper()
	for _, h := range filtered {
		if h.Alias == alias {
			t.Errorf("expected filtered hosts NOT to contain %q", alias)
			return
		}
	}
}

func requireSelected(t *testing.T, d *DropdownState) sshconfig.Host {
	t.Helper()
	selected, ok := d.Selected()
	if !ok {
		t.Fatal("expected a selection")
	}
	return selected
}

func assertSelectedAlias(t *testing.T, d *DropdownState, want string) {
	t.Helper()
	selected := requireSelected(t, d)
	if selected.Alias != want {
		t.Errorf("selected alias = %q, want %q", selected.Alias, want)
	}
}

func assertViewContains(t *testing.T, view, substr string) {
	t.Helper()
	if !strings.Contains(view, substr) {
		t.Errorf("expected view to contain %q", substr)
	}
}

func TestDropdown_Filter(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)

	// Open with "at" filter — should match "atlas" (alias contains "at") but not "forge".
	d.Open("at")

	if !d.Active {
		t.Fatal("dropdown should be active after Open")
	}
	if len(d.filtered) != 1 {
		t.Errorf("expected 1 filtered host (atlas), got %d", len(d.filtered))
	}

	assertFilteredContains(t, d.filtered, "atlas")
	assertFilteredExcludes(t, d.filtered, "forge")
}

func TestDropdown_Select(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)
	d.Open("")

	// Initially cursor is at 0 (atlas).
	assertSelectedAlias(t, &d, "atlas")

	// Move down to forge.
	d.MoveDown()
	assertSelectedAlias(t, &d, "forge")

	// Move down again to bastion.
	d.MoveDown()
	assertSelectedAlias(t, &d, "bastion")

	// Move up to forge.
	d.MoveUp()
	assertSelectedAlias(t, &d, "forge")
}

func TestDropdown_EscDismisses(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)

	d.Open("")
	if !d.Active {
		t.Fatal("dropdown should be active after Open")
	}

	d.Close()
	if d.Active {
		t.Error("dropdown should be inactive after Close")
	}

	_, ok := d.Selected()
	if ok {
		t.Error("Selected() should return false when dropdown is closed")
	}
}

func TestDropdown_Toggle(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)

	// Toggle on.
	d.Toggle("")
	if !d.Active {
		t.Error("dropdown should be active after first Toggle")
	}

	// Toggle off.
	d.Toggle("")
	if d.Active {
		t.Error("dropdown should be inactive after second Toggle")
	}
}

func TestMetadataPreview_Shown(t *testing.T) {
	hosts := testHostsComplete()

	// Prefix match on "atlas".
	result := renderHostMetadata("atlas", hosts)
	if result == "" {
		t.Fatal("expected metadata preview, got empty string")
	}

	if !strings.Contains(result, "atlas") {
		t.Error("expected metadata to contain alias 'atlas'")
	}
	if !strings.Contains(result, "192.168.1.50") {
		t.Error("expected metadata to contain hostname '192.168.1.50'")
	}
	if !strings.Contains(result, "→") {
		t.Error("expected metadata to contain arrow separator '→'")
	}
}

func TestMetadataPreview_PartialMatch(t *testing.T) {
	hosts := testHostsComplete()

	// Partial prefix match on "at" should match "atlas".
	result := renderHostMetadata("at", hosts)
	if result == "" {
		t.Fatal("expected metadata preview for partial match, got empty string")
	}

	if !strings.Contains(result, "atlas") {
		t.Error("expected partial match 'at' to show 'atlas'")
	}
}

func TestMetadataPreview_NoMatch(t *testing.T) {
	hosts := testHostsComplete()

	result := renderHostMetadata("nonexistent", hosts)
	if result != "" {
		t.Errorf("expected empty string for no match, got %q", result)
	}
}

func TestMetadataPreview_EmptyInput(t *testing.T) {
	hosts := testHostsComplete()

	result := renderHostMetadata("", hosts)
	if result != "" {
		t.Error("expected empty string for empty input")
	}
}

func TestFilterHosts_Empty(t *testing.T) {
	hosts := testHostsComplete()

	result := filterHosts(hosts, "")
	if len(result) != len(hosts) {
		t.Errorf("expected empty query to return all %d hosts, got %d", len(hosts), len(result))
	}
}

func TestFilterHosts_MatchAlias(t *testing.T) {
	hosts := testHostsComplete()

	result := filterHosts(hosts, "forge")
	if len(result) != 1 {
		t.Fatalf("expected 1 match for 'forge', got %d", len(result))
	}
	if result[0].Alias != "forge" {
		t.Errorf("expected 'forge', got %s", result[0].Alias)
	}
}

func TestFilterHosts_MatchHostName(t *testing.T) {
	hosts := testHostsComplete()

	result := filterHosts(hosts, "example.com")
	if len(result) != 1 {
		t.Fatalf("expected 1 match for 'example.com', got %d", len(result))
	}
	if result[0].Alias != "forge" {
		t.Errorf("expected 'forge' (hostname match), got %s", result[0].Alias)
	}
}

func TestFilterHosts_MatchUser(t *testing.T) {
	hosts := testHostsComplete()

	result := filterHosts(hosts, "alice")
	if len(result) != 1 {
		t.Fatalf("expected 1 match for 'alice' (user), got %d", len(result))
	}
	if result[0].Alias != "atlas" {
		t.Errorf("expected 'atlas' (user match), got %s", result[0].Alias)
	}
}

func TestFilterHosts_CaseInsensitive(t *testing.T) {
	hosts := testHostsComplete()

	result := filterHosts(hosts, "ATLAS")
	if len(result) != 1 {
		t.Fatalf("expected 1 match for 'ATLAS' (case-insensitive), got %d", len(result))
	}
	if result[0].Alias != "atlas" {
		t.Errorf("expected 'atlas', got %s", result[0].Alias)
	}
}

func TestFilterHosts_MultipleMatches(t *testing.T) {
	hosts := testHostsComplete()

	// "a" matches "atlas" (alias), "bastion" (alias), and admin (user).
	result := filterHosts(hosts, "a")
	if len(result) != 3 {
		t.Errorf("expected 3 matches for 'a', got %d", len(result))
	}
}

func TestDropdown_View(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)
	d.Open("")

	view := d.View([]string{})
	if view == "" {
		t.Fatal("expected non-empty view")
	}

	assertViewContains(t, view, "atlas")
	assertViewContains(t, view, "forge")
	assertViewContains(t, view, "bastion")
	assertViewContains(t, view, "▸")
}

func TestDropdown_ViewWithExisting(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)

	d.Open("")

	// Mark "atlas" as already added.
	view := d.View([]string{"atlas"})
	if !strings.Contains(view, "(added)") {
		t.Error("expected view to contain '(added)' tag for existing host")
	}
}

func TestDropdown_ViewInactive(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)

	// Don't open the dropdown.
	view := d.View([]string{})
	if view != "" {
		t.Error("expected empty view when dropdown is inactive")
	}
}

func TestDropdown_ViewEmpty(t *testing.T) {
	hosts := testHostsComplete()
	d := NewDropdown(hosts)

	// Open with a filter that matches nothing.
	d.Open("zzzzz")

	view := d.View([]string{})
	if view != "" {
		t.Error("expected empty view when no hosts match filter")
	}
}
