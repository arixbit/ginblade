package i18n

import (
	"sync"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"en.json": {Data: []byte(`{
			"errors": {"UNAUTHORIZED": "unauthorized", "DATABASE_ERROR": "database error", "default": "operation failed"},
			"validation": {"required": "{field} is required", "min": "{field} must be at least {param}", "default": "{field} is invalid"}
		}`)},
		"zh-CN.json": {Data: []byte(`{
			"errors": {"UNAUTHORIZED": "未认证", "default": "操作失败"},
			"validation": {"required": "{field} 不能为空"}
		}`)},
		"README.md": {Data: []byte("not a catalog")},
	}
}

func newTestTranslator(t *testing.T) *Translator {
	t.Helper()
	tr, err := New(testFS(), "en")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tr
}

func TestNewErrors(t *testing.T) {
	valid := []byte(`{"errors": {}}`)
	tests := []struct {
		name          string
		fsys          fstest.MapFS
		defaultLocale string
	}{
		{"default locale missing", testFS(), "fr"},
		{"no catalogs", fstest.MapFS{}, "en"},
		{"malformed json", fstest.MapFS{"en.json": {Data: []byte(`{`)}}, "en"},
		{"non-string message", fstest.MapFS{"en.json": {Data: []byte(`{"errors": {"A": 1}}`)}}, "en"},
		{"invalid locale name", fstest.MapFS{"en.json": {Data: valid}, "messages.json": {Data: valid}}, "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.fsys, tt.defaultLocale); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLocalizerMatch(t *testing.T) {
	tr := newTestTranslator(t)
	tests := []struct {
		name  string
		prefs []string
		want  string
	}{
		{"no preferences", nil, "en"},
		{"empty preferences", []string{"", ""}, "en"},
		{"exact tag", []string{"zh-CN"}, "zh-CN"},
		{"lowercase tag", []string{"zh-cn"}, "zh-CN"},
		{"base language", []string{"zh"}, "zh-CN"},
		{"closest script variant", []string{"zh-TW"}, "zh-CN"},
		{"regional variant of default", []string{"en-GB"}, "en"},
		{"accept-language weights", []string{"en;q=0.5,zh-CN;q=0.9"}, "zh-CN"},
		{"unsupported language", []string{"fr-FR"}, "en"},
		{"unsupported first entry in header", []string{"fr,zh-CN;q=0.8"}, "zh-CN"},
		{"first matching preference wins", []string{"en", "zh-CN"}, "en"},
		{"unmatched preference falls through", []string{"fr", "zh-CN"}, "zh-CN"},
		{"malformed preference", []string{"!!!"}, "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tr.Localizer(tt.prefs...).locale; got != tt.want {
				t.Fatalf("Localizer(%q) locale = %q, want %q", tt.prefs, got, tt.want)
			}
		})
	}
}

func TestLocalizerError(t *testing.T) {
	tr := newTestTranslator(t)
	tests := []struct {
		name   string
		locale string
		reason string
		want   string
	}{
		{"hit", "zh-CN", "UNAUTHORIZED", "未认证"},
		{"missing key uses default locale", "zh-CN", "DATABASE_ERROR", "database error"},
		{"unknown key uses locale fallback", "zh-CN", "SOMETHING_ELSE", "操作失败"},
		{"default locale fallback", "en", "SOMETHING_ELSE", "operation failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tr.Localizer(tt.locale).Error(tt.reason); got != tt.want {
				t.Fatalf("Error(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}

func TestLocalizerValidation(t *testing.T) {
	tr := newTestTranslator(t)
	tests := []struct {
		name   string
		locale string
		tag    string
		param  string
		want   string
	}{
		{"hit", "zh-CN", "required", "", "name 不能为空"},
		{"param placeholder", "en", "min", "3", "name must be at least 3"},
		{"missing key uses default locale", "zh-CN", "min", "3", "name must be at least 3"},
		{"unknown tag uses default template", "en", "email", "", "name is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tr.Localizer(tt.locale).Validation(tt.tag, "name", tt.param); got != tt.want {
				t.Fatalf("Validation(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestTranslatorConcurrentUse(t *testing.T) {
	tr := newTestTranslator(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := tr.Localizer("zh-CN,en;q=0.5").Error("UNAUTHORIZED"); got != "未认证" {
				t.Errorf("Error = %q, want 未认证", got)
			}
		})
	}
	wg.Wait()
}

func TestZeroLocalizerReturnsKeys(t *testing.T) {
	var l Localizer
	if got := l.Error("UNAUTHORIZED"); got != "UNAUTHORIZED" {
		t.Errorf("Error = %q, want the reason itself", got)
	}
	if got := l.Validation("required", "name", ""); got != "required" {
		t.Errorf("Validation = %q, want the tag itself", got)
	}
}
