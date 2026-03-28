package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ViciousSyndicateSource struct {
	indexURL string
	client   *http.Client
}

type viciousSyndicateSnapshotPayload struct {
	ReportTitle string                 `json:"report_title"`
	ReportURL   string                 `json:"report_url"`
	Decks       []viciousSyndicateDeck `json:"decks,omitempty"`
}

type viciousSyndicateDeck struct {
	ExternalRef string   `json:"external_ref"`
	Name        string   `json:"name"`
	Class       string   `json:"class"`
	Format      string   `json:"format"`
	Cards       []string `json:"cards,omitempty"`
}

var (
	vsReportLinkPattern  = regexp.MustCompile(`https?://[^"' ]+/vs-data-reaper-report-\d+/|/vs-data-reaper-report-\d+/`)
	vsDeckLibraryPattern = regexp.MustCompile(`https?://[^"' ]+/deck-library/[^"' ]+/|/deck-library/[^"' ]+/`)
	vsDeckVariantPattern = regexp.MustCompile(`https?://[^"' ]+/decks/[^"' ]+/|/decks/[^"' ]+/`)
	vsTitlePattern       = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	vsTimePattern        = regexp.MustCompile(`datetime="([^"]+)"`)
	vsClassFormatPattern = regexp.MustCompile(`CLASS:\s*([^|<]+)\s*\|\s*Format:\s*([^|<]+)`)
	vsCardLinePattern    = regexp.MustCompile(`>\s*(\d+)\s+([^<]+?)\s+\d+\s+CORE\s*<`)
)

func NewViciousSyndicateSource(indexURL string) ViciousSyndicateSource {
	return ViciousSyndicateSource{
		indexURL: indexURL,
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (s ViciousSyndicateSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	indexHTML, err := s.fetchText(ctx, s.indexURL)
	if err != nil {
		return FetchResult{}, err
	}

	reportURL := resolveVSReportURL(s.indexURL, firstMatch(indexHTML, vsReportLinkPattern))
	if reportURL == "" {
		return FetchResult{}, fmt.Errorf("find latest vicious syndicate report link: no report link found")
	}

	reportHTML, err := s.fetchText(ctx, reportURL)
	if err != nil {
		return FetchResult{}, err
	}

	title := cleanHTMLText(firstSubmatch(reportHTML, vsTitlePattern))
	reportTime := time.Now().UTC()
	if rawTime := firstSubmatch(reportHTML, vsTimePattern); rawTime != "" {
		if parsed, err := time.Parse(time.RFC3339, rawTime); err == nil {
			reportTime = parsed
		}
	}

	rawPayload, err := json.Marshal(viciousSyndicateSnapshotPayload{
		ReportTitle: title,
		ReportURL:   reportURL,
		Decks:       s.extractDecks(ctx, reportHTML, reportURL),
	})
	if err != nil {
		return FetchResult{}, fmt.Errorf("encode vicious syndicate payload: %w", err)
	}

	return FetchResult{
		Source:       "vicioussyndicate",
		PatchVersion: title,
		Format:       "standard",
		FetchedAt:    reportTime,
		RawPayload:   string(rawPayload),
	}, nil
}

func (s ViciousSyndicateSource) extractDecks(ctx context.Context, reportHTML string, reportURL string) []viciousSyndicateDeck {
	libraryLinks := uniqueResolvedLinks(reportHTML, reportURL, vsDeckLibraryPattern)
	decks := make([]viciousSyndicateDeck, 0)
	for _, libraryURL := range libraryLinks {
		libraryHTML, err := s.fetchText(ctx, libraryURL)
		if err != nil {
			continue
		}

		variantLinks := uniqueResolvedLinks(libraryHTML, libraryURL, vsDeckVariantPattern)
		for _, variantURL := range variantLinks {
			variantHTML, err := s.fetchText(ctx, variantURL)
			if err != nil {
				continue
			}

			name := cleanHTMLText(firstSubmatch(variantHTML, vsTitlePattern))
			className := "UNKNOWN"
			format := "standard"
			if matches := vsClassFormatPattern.FindStringSubmatch(variantHTML); len(matches) >= 3 {
				className = strings.ToUpper(strings.TrimSpace(matches[1]))
				format = strings.ToLower(strings.TrimSpace(matches[2]))
			}

			cardMatches := vsCardLinePattern.FindAllStringSubmatch(variantHTML, -1)
			cards := make([]string, 0, len(cardMatches))
			for _, match := range cardMatches {
				if len(match) >= 3 {
					cards = append(cards, fmt.Sprintf("%sx %s", strings.TrimSpace(match[1]), cleanHTMLText(match[2])))
				}
			}

			decks = append(decks, viciousSyndicateDeck{
				ExternalRef: variantURL,
				Name:        name,
				Class:       className,
				Format:      format,
				Cards:       cards,
			})
		}
	}

	return decks
}

func (s ViciousSyndicateSource) fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build vicious syndicate request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch vicious syndicate page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch vicious syndicate page: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read vicious syndicate page: %w", err)
	}
	return string(body), nil
}

func firstMatch(input string, pattern *regexp.Regexp) string {
	return pattern.FindString(input)
}

func firstSubmatch(input string, pattern *regexp.Regexp) string {
	matches := pattern.FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func resolveVSReportURL(indexURL, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base := strings.TrimRight(indexURL, "/")
	if idx := strings.Index(base, "/tag/meta"); idx >= 0 {
		base = base[:idx]
	}
	return base + href
}

func uniqueResolvedLinks(input string, baseURL string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllString(input, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		resolved := resolveVSReportURL(baseURL, match)
		if resolved == "" {
			continue
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		out = append(out, resolved)
	}
	return out
}

func cleanHTMLText(input string) string {
	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	return strings.Join(strings.Fields(html.UnescapeString(replacer.Replace(input))), " ")
}
