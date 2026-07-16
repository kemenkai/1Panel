package service

import (
	"sort"
	"testing"

	"github.com/1Panel-dev/1Panel/core/app/dto"
)

func TestFilterWindowsLiteMenus(t *testing.T) {
	// Full top-level menu tree as produced by helper.LoadMenus().
	input := []dto.ShowMenu{
		{Label: "Home-Menu"},
		{Label: "App-Menu"},
		{Label: "AI-Menu"},
		{Label: "Website-Menu"},
		{Label: "Database-Menu"},
		{Label: "Container-Menu"},
		{Label: "System-Menu"},
		{Label: "Terminal-Menu"},
		{Label: "Cronjob-Menu"},
		{Label: "Toolbox-Menu"},
		{Label: "Xpack-Menu"},
		{Label: "Log-Menu"},
		{Label: "Setting-Menu"},
		{Label: "Enhance-Menu"},
	}

	want := map[string]struct{}{
		"Home-Menu":      {},
		"Enhance-Menu":   {},
		"Container-Menu": {},
		"Toolbox-Menu":   {},
		"Log-Menu":       {},
		"Setting-Menu":   {},
	}

	got := filterWindowsLiteMenus(input)

	if len(got) != len(want) {
		t.Fatalf("expected %d menus, got %d: %+v", len(want), len(got), labelsOf(got))
	}

	seen := make(map[string]struct{})
	for _, m := range got {
		if _, ok := want[m.Label]; !ok {
			t.Errorf("unexpected menu leaked into Windows sidebar: %q", m.Label)
		}
		seen[m.Label] = struct{}{}
	}
	for label := range want {
		if _, ok := seen[label]; !ok {
			t.Errorf("expected allowed menu %q to be retained, but it was dropped", label)
		}
	}
}

func TestFilterWindowsLiteMenusEmpty(t *testing.T) {
	if got := filterWindowsLiteMenus(nil); len(got) != 0 {
		t.Fatalf("expected empty result for nil input, got %+v", labelsOf(got))
	}
}

func labelsOf(menus []dto.ShowMenu) []string {
	out := make([]string, 0, len(menus))
	for _, m := range menus {
		out = append(out, m.Label)
	}
	sort.Strings(out)
	return out
}
