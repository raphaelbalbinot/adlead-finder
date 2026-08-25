package meta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// RawAd representa o formato bruto de um anúncio retornado pela Graph API da Meta
type RawAd struct {
	ID                         string   `json:"id"`
	PageID                     string   `json:"page_id"`
	PageName                   string   `json:"page_name"`
	AdCreativeBodies           []string `json:"ad_creative_bodies"`
	AdSnapshotURL              string   `json:"ad_snapshot_url"`
	AdCreativeLinkCaptions     []string `json:"ad_creative_link_captions"`
	AdCreativeLinkTitles       []string `json:"ad_creative_link_titles"`
	AdCreativeLinkDescriptions []string `json:"ad_creative_link_descriptions"`
	PublisherPlatforms         []string `json:"publisher_platforms"`
	AdDeliveryStartTime        string   `json:"ad_delivery_start_time"`
	AdCreationTime             string   `json:"ad_creation_time"`
}

// MetaAPIResponse encapsula a resposta da Graph API
type MetaAPIResponse struct {
	Data   []RawAd `json:"data"`
	Paging *struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
		Next string `json:"next"`
	} `json:"paging,omitempty"`
	Error *struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		ErrorSub  int    `json:"error_subcode"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error,omitempty"`
}

// AggregatedCompany agrupa todos os anúncios ativos sob a mesma página/empresa
type AggregatedCompany struct {
	PageID              string   `json:"page_id"`
	CompanyName         string   `json:"company_name"`
	ActiveAdsCount      int      `json:"active_ads_count"`
	AdCreativeSample    string   `json:"ad_creative_sample"`
	AdSnapshotURL       string   `json:"ad_snapshot_url"`
	LandingPageURL      string   `json:"landing_page_url"`
	PublisherPlatforms  []string `json:"publisher_platforms"`
	AdDeliveryStartTime string   `json:"ad_delivery_start_time"`
	AdCreationTime      string   `json:"ad_creation_time"`
	DaysRunning         int      `json:"days_running"`
}

// SearchParams define os filtros da busca
type SearchParams struct {
	SearchTerms        string   `json:"search_terms"`
	Limit              int      `json:"limit"`
	AdDeliveryDateMin  string   `json:"ad_delivery_date_min"` // formato YYYY-MM-DD
	PublisherPlatforms []string `json:"publisher_platforms"`   // ex: ["FACEBOOK", "INSTAGRAM"]
}

// Client gerencia chamadas à Meta Ad Library API
type Client struct {
	accessToken string
	httpClient  *http.Client
}

// NewClient cria uma nova instância do cliente Meta
func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// FetchAdsPage busca uma página individual de anúncios na Meta Ads Library retornando as empresas agregadas e o cursor after
func (c *Client) FetchAdsPage(params SearchParams, afterCursor string) ([]AggregatedCompany, string, error) {
	if c.accessToken == "" {
		return nil, "", fmt.Errorf("token de acesso da Meta não configurado")
	}

	endpoint := "https://graph.facebook.com/v21.0/ads_archive"
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, "", err
	}

	cleanToken := strings.TrimSpace(c.accessToken)
	q := reqURL.Query()
	q.Set("access_token", cleanToken)
	q.Set("ad_type", "ALL")
	q.Set("ad_active_status", "ACTIVE")
	q.Set("search_terms", params.SearchTerms)
	q.Set("ad_reached_countries", "['BR']")
	q.Set("fields", "id,page_id,page_name,ad_creative_bodies,ad_snapshot_url,ad_creative_link_captions,ad_creative_link_titles,ad_creative_link_descriptions,publisher_platforms,ad_delivery_start_time,ad_creation_time")
	q.Set("limit", "50")

	if afterCursor != "" {
		q.Set("after", afterCursor)
	}

	if params.AdDeliveryDateMin != "" {
		q.Set("ad_delivery_date_min", params.AdDeliveryDateMin)
	}

	if len(params.PublisherPlatforms) > 0 {
		var validPlatforms []string
		for _, p := range params.PublisherPlatforms {
			pUpper := strings.ToUpper(strings.TrimSpace(p))
			if pUpper != "" && pUpper != "TODAS" && pUpper != "ALL" {
				validPlatforms = append(validPlatforms, fmt.Sprintf("'%s'", pUpper))
			}
		}
		if len(validPlatforms) > 0 {
			q.Set("publisher_platforms", "["+strings.Join(validPlatforms, ",")+"]")
		}
	}

	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("erro ao criar requisição para Meta API: %w", err)
	}
	req.Header.Set("User-Agent", "AdLeadFinder/1.0")
	req.Header.Set("Authorization", "Bearer "+cleanToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("erro ao conectar com a Meta Graph API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var metaResp MetaAPIResponse
	if err := json.Unmarshal(bodyBytes, &metaResp); err != nil {
		return nil, "", err
	}

	if metaResp.Error != nil {
		return nil, "", fmt.Errorf("erro retornado pela Meta API: %s (código: %d)", metaResp.Error.Message, metaResp.Error.Code)
	}

	nextCursor := ""
	if metaResp.Paging != nil && metaResp.Paging.Cursors.After != "" && metaResp.Paging.Next != "" {
		nextCursor = metaResp.Paging.Cursors.After
	}

	companies := c.aggregateAds(metaResp.Data)
	return companies, nextCursor, nil
}

var urlRegex = regexp.MustCompile(`(?i)\bhttps?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(?:/[^\s]*)?`)

func (c *Client) aggregateAds(ads []RawAd) []AggregatedCompany {
	companiesMap := make(map[string]*AggregatedCompany)
	var order []string

	for _, ad := range ads {
		pageID := strings.TrimSpace(ad.PageID)
		if pageID == "" {
			continue
		}

		comp, exists := companiesMap[pageID]
		if !exists {
			comp = &AggregatedCompany{
				PageID:              pageID,
				CompanyName:         strings.TrimSpace(ad.PageName),
				ActiveAdsCount:      0,
				AdSnapshotURL:       ad.AdSnapshotURL,
				PublisherPlatforms:  ad.PublisherPlatforms,
				AdDeliveryStartTime: strings.TrimSpace(ad.AdDeliveryStartTime),
				AdCreationTime:      strings.TrimSpace(ad.AdCreationTime),
			}
			companiesMap[pageID] = comp
			order = append(order, pageID)
		}

		comp.ActiveAdsCount++

		// Preserva a data de início mais antiga entre os anúncios da mesma empresa
		if ad.AdDeliveryStartTime != "" {
			if comp.AdDeliveryStartTime == "" || isEarlier(ad.AdDeliveryStartTime, comp.AdDeliveryStartTime) {
				comp.AdDeliveryStartTime = strings.TrimSpace(ad.AdDeliveryStartTime)
			}
		}
		if ad.AdCreationTime != "" {
			if comp.AdCreationTime == "" || isEarlier(ad.AdCreationTime, comp.AdCreationTime) {
				comp.AdCreationTime = strings.TrimSpace(ad.AdCreationTime)
			}
		}

		// Amostra de criativo (mantém o maior ou mais detalhado)
		for _, body := range ad.AdCreativeBodies {
			bodyClean := strings.TrimSpace(body)
			if len(bodyClean) > len(comp.AdCreativeSample) {
				comp.AdCreativeSample = bodyClean
			}
		}

		// Tentativa de extrair URL de destino da Landing Page
		if comp.LandingPageURL == "" {
			comp.LandingPageURL = extractLandingPageURL(ad)
		}
	}

	var result []AggregatedCompany
	for _, pageID := range order {
		comp := companiesMap[pageID]
		comp.DaysRunning = calculateDaysRunning(comp.AdDeliveryStartTime, comp.AdCreationTime)
		result = append(result, *comp)
	}
	return result
}

func parseMetaDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("data vazia")
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("formato de data desconhecido: %s", dateStr)
}

func isEarlier(dateStrA, dateStrB string) bool {
	tA, errA := parseMetaDate(dateStrA)
	tB, errB := parseMetaDate(dateStrB)
	if errA != nil || errB != nil {
		return false
	}
	return tA.Before(tB)
}

func calculateDaysRunning(startTimeStr, creationTimeStr string) int {
	dateStr := strings.TrimSpace(startTimeStr)
	if dateStr == "" {
		dateStr = strings.TrimSpace(creationTimeStr)
	}
	if dateStr == "" {
		return 0
	}

	parsedTime, err := parseMetaDate(dateStr)
	if err != nil || parsedTime.IsZero() {
		return 0
	}

	diff := time.Since(parsedTime)
	days := int(diff.Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func extractLandingPageURL(ad RawAd) string {
	// 1. Verifica nos captions
	for _, cap := range ad.AdCreativeLinkCaptions {
		cap = strings.TrimSpace(cap)
		if cap != "" {
			if strings.HasPrefix(cap, "http://") || strings.HasPrefix(cap, "https://") {
				return cleanURL(cap)
			}
			if strings.Contains(cap, ".") && !strings.Contains(cap, " ") {
				return "https://" + cleanURL(cap)
			}
		}
	}

	// 2. Procura links explícitos nos textos dos corpos do anúncio
	for _, body := range ad.AdCreativeBodies {
		match := urlRegex.FindString(body)
		if match != "" && !strings.Contains(match, "facebook.com") && !strings.Contains(match, "instagram.com") {
			return cleanURL(match)
		}
	}

	// 3. Procura nos títulos dos links
	for _, title := range ad.AdCreativeLinkTitles {
		match := urlRegex.FindString(title)
		if match != "" {
			return cleanURL(match)
		}
	}

	return ""
}

func cleanURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, ".,;!?")
	return raw
}
