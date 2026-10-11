// Package i18n loads per-locale message catalogs and picks the best locale for a request.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"golang.org/x/text/language"
)

const (
	sectionErrors     = "errors"
	sectionValidation = "validation"
	// fallbackKey names the entry used when a section has no message for a key.
	fallbackKey = "default"
)

// catalog maps section -> key -> message for one locale.
type catalog map[string]map[string]string

// Translator holds every loaded catalog. It is read-only after New and safe for concurrent use.
type Translator struct {
	defaultLocale string
	locales       []string // matcher order, default locale first
	catalogs      map[string]catalog
	matcher       language.Matcher
}

// New loads each <locale>.json at the root of fsys, where <locale> is a BCP 47 tag
// such as "en" or "zh-CN". defaultLocale must be one of the loaded locales.
func New(fsys fs.FS, defaultLocale string) (*Translator, error) {
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, fmt.Errorf("list catalogs: %w", err)
	}

	catalogs := make(map[string]catalog, len(names))
	var others []string
	for _, name := range names {
		locale := strings.TrimSuffix(name, ".json")
		if _, err := language.Parse(locale); err != nil {
			return nil, fmt.Errorf("catalog %s: invalid locale: %w", name, err)
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read catalog %s: %w", name, err)
		}
		var cat catalog
		if err := json.Unmarshal(data, &cat); err != nil {
			return nil, fmt.Errorf("parse catalog %s: %w", name, err)
		}
		catalogs[locale] = cat
		if locale != defaultLocale {
			others = append(others, locale)
		}
	}
	if _, ok := catalogs[defaultLocale]; !ok {
		return nil, fmt.Errorf("default locale %q has no catalog", defaultLocale)
	}

	// The matcher falls back to its first tag, so the default locale goes first.
	locales := append([]string{defaultLocale}, others...)
	tags := make([]language.Tag, len(locales))
	for i, locale := range locales {
		tags[i] = language.Make(locale)
	}

	return &Translator{
		defaultLocale: defaultLocale,
		locales:       locales,
		catalogs:      catalogs,
		matcher:       language.NewMatcher(tags),
	}, nil
}

// Localizer returns a Localizer for the first preference that matches a loaded
// locale, or for the default locale when none does. Each preference is a tag
// ("zh-CN") or an Accept-Language value ("zh-CN,zh;q=0.9"); empty ones are skipped.
func (t *Translator) Localizer(prefs ...string) Localizer {
	_, index := language.MatchStrings(t.matcher, prefs...)
	return Localizer{t: t, locale: t.locales[index]}
}

// lookup tries key and then fallbackKey, each in locale and then in the default
// locale. It returns key itself when nothing matches, including on a nil Translator.
func (t *Translator) lookup(locale, section, key string) string {
	if t == nil {
		return key
	}
	for _, k := range [...]string{key, fallbackKey} {
		for _, loc := range [...]string{locale, t.defaultLocale} {
			if msg, ok := t.catalogs[loc][section][k]; ok {
				return msg
			}
		}
	}
	return key
}

// Localizer resolves messages in one locale. Its zero value has no catalogs and
// returns keys unchanged.
type Localizer struct {
	t      *Translator
	locale string
}

// Error returns the message for an errcode reason such as "INVALID_PARAMS".
func (l Localizer) Error(reason string) string {
	return l.t.lookup(l.locale, sectionErrors, reason)
}

// Validation returns the message for a failed validator tag such as "required",
// with {field} and {param} filled in.
func (l Localizer) Validation(tag, field, param string) string {
	msg := l.t.lookup(l.locale, sectionValidation, tag)
	return strings.NewReplacer("{field}", field, "{param}", param).Replace(msg)
}
