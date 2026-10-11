package locales

import (
	"encoding/json"
	"io/fs"
	"slices"
	"testing"

	"github.com/arixbit/ginblade/pkg/i18n"
)

func TestCatalogsLoad(t *testing.T) {
	if _, err := i18n.New(FS, "en"); err != nil {
		t.Fatalf("i18n.New: %v", err)
	}
}

// TestCatalogsHaveSameKeys fails when a locale is missing a message that
// en.json has, or has one en.json lacks, so gaps surface in CI rather than as
// silent fallbacks in production.
func TestCatalogsHaveSameKeys(t *testing.T) {
	want := catalogKeys(t, "en.json")
	names, err := fs.Glob(FS, "*.json")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, name := range names {
		got := catalogKeys(t, name)
		if missing := difference(want, got); len(missing) > 0 {
			t.Errorf("%s is missing %v", name, missing)
		}
		if extra := difference(got, want); len(extra) > 0 {
			t.Errorf("%s has keys en.json lacks: %v", name, extra)
		}
	}
}

// catalogKeys returns the catalog's keys as "section.key".
func catalogKeys(t *testing.T, name string) map[string]struct{} {
	t.Helper()
	data, err := fs.ReadFile(FS, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var cat map[string]map[string]string
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	keys := make(map[string]struct{})
	for section, msgs := range cat {
		for key := range msgs {
			keys[section+"."+key] = struct{}{}
		}
	}
	return keys
}

// difference returns the sorted keys in a that are not in b.
func difference(a, b map[string]struct{}) []string {
	var out []string
	for key := range a {
		if _, ok := b[key]; !ok {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}
