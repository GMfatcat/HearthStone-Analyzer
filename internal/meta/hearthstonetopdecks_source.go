package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type HearthstoneTopDecksSource struct {
	indexURL string
	client   *http.Client
}

type hearthstoneTopDecksSnapshotPayload struct {
	ReportTitle string                   `json:"report_title"`
	ReportURL   string                   `json:"report_url"`
	Decks       []hearthstoneTopDeckDeck `json:"decks,omitempty"`
}

type hearthstoneTopDeckDeck struct {
	ExternalRef string   `json:"external_ref"`
	Name        string   `json:"name"`
	Class       string   `json:"class"`
	Format      string   `json:"format"`
	Tier        *string  `json:"tier,omitempty"`
	Cards       []string `json:"cards,omitempty"`
}

var (
	htdTitlePattern       = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	htdTierHeadingPattern = regexp.MustCompile(`(?is)<h[1-6][^>]*>\s*Tier\s*([1-4])[^<]*</h[1-6]>`)
	htdDeckAnchorPattern  = regexp.MustCompile(`(?is)<a[^>]+href="([^"]+)"[^>]*class="[^"]*class-header\s+([^"]+)-header[^"]*"[^>]*>\s*<h2[^>]*>([^<]*?Standard Meta Tier List[^<]*)</h2>\s*</a>`)
	htdCardLinePattern    = regexp.MustCompile(`(?is)<li[^>]*>.*?<span[^>]*class="card-cost"[^>]*>\s*\d+\s*</span>\s*<a[^>]*>\s*<span[^>]*class="card-name"[^>]*>\s*([^<]+?)\s*</span>\s*</a>\s*<span[^>]*class="card-count"[^>]*>\s*(\d+)\s*</span>`)
	htdMetaSuffixPattern  = regexp.MustCompile(`(?i)\s*[-–]\s*Standard Meta Tier List.*$`)
	htdSideboardPattern   = regexp.MustCompile(`(?is)<h[1-6][^>]*>\s*Sideboard\s*</h[1-6]>`)
)

func NewHearthstoneTopDecksSource(indexURL string) HearthstoneTopDecksSource {
	return HearthstoneTopDecksSource{
		indexURL: indexURL,
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (s HearthstoneTopDecksSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	html, err := s.fetchText(ctx, s.indexURL)
	if err != nil {
		return FetchResult{}, err
	}

	title := cleanHTMLText(firstSubmatch(html, htdTitlePattern))
	fetchedAt := time.Now().UTC()
	rawPayload, err := json.Marshal(hearthstoneTopDecksSnapshotPayload{
		ReportTitle: title,
		ReportURL:   s.indexURL,
		Decks:       extractHTDDecks(s.indexURL, html),
	})
	if err != nil {
		return FetchResult{}, fmt.Errorf("encode hearthstone top decks payload: %w", err)
	}

	return FetchResult{
		Source:       "hearthstonetopdecks",
		PatchVersion: firstNonEmpty(title, "Hearthstone Top Decks Standard Meta"),
		Format:       "standard",
		FetchedAt:    fetchedAt,
		RawPayload:   string(rawPayload),
	}, nil
}

func extractHTDDecks(baseURL, html string) []hearthstoneTopDeckDeck {
	tierMatches := htdTierHeadingPattern.FindAllStringSubmatchIndex(html, -1)
	if len(tierMatches) == 0 {
		return nil
	}

	decks := make([]hearthstoneTopDeckDeck, 0)
	for idx, match := range tierMatches {
		if len(match) < 4 {
			continue
		}
		tierLabel := "T" + html[match[2]:match[3]]
		sectionStart := match[1]
		sectionEnd := len(html)
		if idx+1 < len(tierMatches) && len(tierMatches[idx+1]) > 0 {
			sectionEnd = tierMatches[idx+1][0]
		}
		sectionDecks := extractHTDDecksFromSection(baseURL, html[sectionStart:sectionEnd], tierLabel)
		decks = append(decks, sectionDecks...)
	}

	return decks
}

func extractHTDDecksFromSection(baseURL, sectionHTML, tierLabel string) []hearthstoneTopDeckDeck {
	matches := htdDeckAnchorPattern.FindAllStringSubmatchIndex(sectionHTML, -1)
	decks := make([]hearthstoneTopDeckDeck, 0, len(matches))
	for idx, match := range matches {
		if len(match) < 8 {
			continue
		}

		href := sectionHTML[match[2]:match[3]]
		classSlug := sectionHTML[match[4]:match[5]]
		name := cleanHTMLText(sectionHTML[match[6]:match[7]])
		blockStart := match[1]
		blockEnd := len(sectionHTML)
		if idx+1 < len(matches) && len(matches[idx+1]) > 0 {
			blockEnd = matches[idx+1][0]
		}
		blockHTML := sectionHTML[blockStart:blockEnd]
		if sideboardIdx := htdSideboardPattern.FindStringIndex(blockHTML); len(sideboardIdx) == 2 {
			blockHTML = blockHTML[:sideboardIdx[0]]
		}
		cards := extractHTDCards(blockHTML)
		className := inferHTDClass(classSlug, name)

		tier := tierLabel
		decks = append(decks, hearthstoneTopDeckDeck{
			ExternalRef: resolveVSReportURL(baseURL, href),
			Name:        trimHTDDeckTitle(name),
			Class:       className,
			Format:      "standard",
			Tier:        &tier,
			Cards:       cards,
		})
	}
	return decks
}

func extractHTDCards(blockHTML string) []string {
	matches := htdCardLinePattern.FindAllStringSubmatch(blockHTML, -1)
	cards := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		name := cleanHTMLText(match[1])
		count := strings.TrimSpace(match[2])
		if count == "" || name == "" {
			continue
		}
		cards = append(cards, count+"x "+name)
	}
	return cards
}

func trimHTDDeckTitle(name string) string {
	name = htdMetaSuffixPattern.ReplaceAllString(strings.TrimSpace(name), "")
	name = strings.TrimRight(strings.TrimSpace(name), "-– ")
	return name
}

func inferHTDClass(classSlug, name string) string {
	switch strings.ToLower(strings.TrimSpace(classSlug)) {
	case "death-knight":
		return "DEATHKNIGHT"
	case "demon-hunter":
		return "DEMONHUNTER"
	default:
		if classSlug != "" {
			return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(classSlug), "-", ""))
		}
	}

	upper := strings.ToUpper(name)
	for _, className := range []string{
		"DEATH KNIGHT",
		"DEMON HUNTER",
		"DRUID",
		"HUNTER",
		"MAGE",
		"PALADIN",
		"PRIEST",
		"ROGUE",
		"SHAMAN",
		"WARLOCK",
		"WARRIOR",
	} {
		if strings.Contains(upper, className) {
			return strings.ReplaceAll(className, " ", "")
		}
	}
	return "UNKNOWN"
}

func (s HearthstoneTopDecksSource) fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build hearthstone top decks request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch hearthstone top decks page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch hearthstone top decks page: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read hearthstone top decks page: %w", err)
	}
	return string(body), nil
}
