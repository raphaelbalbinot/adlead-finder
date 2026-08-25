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
	ID                         string    `json:"id"`
	PageID                     string    `json:"page_id"`
	PageName                   string    `json:"page_name"`
	AdCreativeBodies           []string  `json:"ad_creative_bodies"`
	AdSnapshotURL              string    `json:"ad_snapshot_url"`
	AdCreativeLinkCaptions     []string  `json:"ad_creative_link_captions"`
	AdCreativeLinkTitles       []string  `json:"ad_creative_link_titles"`
	AdCreativeLinkDescriptions []string  `json:"ad_creative_link_descriptions"`
	PublisherPlatforms         []string  `json:"publisher_platforms"`
	AdDeliveryStartTime        string    `json:"ad_delivery_start_time"`
}

// MetaAPIResponse encapsula a resposta da Graph API
type MetaAPIResponse struct {
	Data   []RawAd `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
		Next string `json:"next"`
	} `json:"paging"`
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
	PageID             string   `json:"page_id"`
	CompanyName        string   `json:"company_name"`
	ActiveAdsCount     int      `json:"active_ads_count"`
	AdCreativeSample   string   `json:"ad_creative_sample"`
	AdSnapshotURL      string   `json:"ad_snapshot_url"`
	LandingPageURL     string   `json:"landing_page_url"`
	PublisherPlatforms []string `json:"publisher_platforms"`
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

// SearchAds busca anúncios na Meta Ad Library e agrupa por page_id
func (c *Client) SearchAds(params SearchParams) ([]AggregatedCompany, error) {
	if c.accessToken == "" {
		return nil, fmt.Errorf("META_ACCESS_TOKEN não está configurado no arquivo .env")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}
	// A API da Meta permite até 100 por página
	if limit > 100 {
		limit = 100
	}

	endpoint := "https://graph.facebook.com/v21.0/ads_archive"
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	q := reqURL.Query()
	q.Set("access_token", c.accessToken)
	q.Set("search_terms", params.SearchTerms)
	q.Set("ad_reached_countries", "['BR']")
	q.Set("ad_active_status", "ACTIVE")
	q.Set("fields", "id,page_id,page_name,ad_creative_bodies,ad_snapshot_url,ad_creative_link_captions,ad_creative_link_titles,ad_creative_link_descriptions,publisher_platforms,ad_delivery_start_time")
	q.Set("limit", fmt.Sprintf("%d", limit))

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
		return nil, fmt.Errorf("erro ao criar requisição para Meta API: %w", err)
	}
	req.Header.Set("User-Agent", "AdLeadFinder/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar com a Meta Graph API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta da Meta API: %w", err)
	}

	var metaResp MetaAPIResponse
	if err := json.Unmarshal(bodyBytes, &metaResp); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON da Meta API: %w", err)
	}

	if metaResp.Error != nil {
		return nil, fmt.Errorf("erro retornado pela Meta API: %s (código: %d)", metaResp.Error.Message, metaResp.Error.Code)
	}

	return c.aggregateAds(metaResp.Data), nil
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
				PageID:             pageID,
				CompanyName:        strings.TrimSpace(ad.PageName),
				ActiveAdsCount:     0,
				AdSnapshotURL:      ad.AdSnapshotURL,
				PublisherPlatforms: ad.PublisherPlatforms,
			}
			companiesMap[pageID] = comp
			order = append(order, pageID)
		}

		comp.ActiveAdsCount++

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
		result = append(result, *companiesMap[pageID])
	}
	return result
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
