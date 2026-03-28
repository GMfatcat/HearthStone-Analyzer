package cards

import (
	"regexp"
	"strings"
)

var (
	cardNameBracketedPattern    = regexp.MustCompile(`\s*[\(\[].*?[\)\]]`)
	cardNameWhitespacePattern   = regexp.MustCompile(`\s+`)
	cardNameNonAlnumPattern     = regexp.MustCompile(`[^a-z0-9]+`)
	cardNamePunctuationReplacer = strings.NewReplacer(
		"\u2019", "'",
		"\u2018", "'",
		"\u201c", "\"",
		"\u201d", "\"",
		"\u2013", "-",
		"\u2014", "-",
		"\u00a0", " ",
	)
)

func LookupNameVariants(name string) []string {
	normalized := normalizeLookupDisplayName(name)
	if normalized == "" {
		return nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, 3)
	appendVariant := func(value string) {
		value = normalizeLookupDisplayName(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	appendVariant(normalized)
	appendVariant(cardNameBracketedPattern.ReplaceAllString(normalized, ""))
	appendVariant(strings.ReplaceAll(normalized, ":", " "))
	return out
}

func LookupNameKeys(name string) []string {
	variants := LookupNameVariants(name)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(variants)*2)
	for _, variant := range variants {
		for _, key := range []string{
			strings.ToLower(strings.TrimSpace(variant)),
			cardNameNonAlnumPattern.ReplaceAllString(strings.ToLower(variant), ""),
		} {
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

func normalizeLookupDisplayName(name string) string {
	name = cardNamePunctuationReplacer.Replace(name)
	name = cardNameWhitespacePattern.ReplaceAllString(strings.TrimSpace(name), " ")
	return strings.TrimSpace(name)
}
