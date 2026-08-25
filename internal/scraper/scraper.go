package scraper

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ExtractedContacts armazena os dados encontrados durante a raspagem da Landing Page
type ExtractedContacts struct {
	WhatsApp  string `json:"whatsapp"`
	Email     string `json:"email"`
	Instagram string `json:"instagram"`
}

// Scraper realiza o scraping concorrente de Landing Pages
type Scraper struct {
	httpClient *http.Client
}

// NewScraper inicializa o Scraper com cliente HTTP configurado
func NewScraper() *Scraper {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Tolera certificados expirados em landing pages locais
	}
	return &Scraper{
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   8 * time.Second,
		},
	}
}

var (
	// Regex para WhatsApp nos links
	waLinkRegex = regexp.MustCompile(`(?i)(?:wa\.me|api\.whatsapp\.com/send\?phone=)(\d{10,15})`)
	
	// Regex para telefones do Brasil no texto HTML
	phoneBRRegex = regexp.MustCompile(`(?:\+?55\s?)?(?:\(?([1-9]{2})\)?\s?)(?:(9\d{4})[-\s]?(\d{4})|(\d{4})[-\s]?(\d{4}))`)
	
	// Regex para emails
	emailRegex = regexp.MustCompile(`(?i)\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
	
	// Regex para Instagram
	instaRegex = regexp.MustCompile(`(?i)https?://(?:www\.)?instagram\.com/([a-zA-Z0-9._-]+)/?`)
)

// ScrapeLandingPage extrai contatos de uma única URL com timeout
func (s *Scraper) ScrapeLandingPage(rawURL string) ExtractedContacts {
	var contacts ExtractedContacts
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || !strings.HasPrefix(rawURL, "http") {
		return contacts
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return contacts
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return contacts
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return contacts
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return contacts
	}

	// 1. Varrer links (href)
	doc.Find("a").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}
		href = strings.TrimSpace(href)

		// WhatsApp em link
		if contacts.WhatsApp == "" {
			if match := waLinkRegex.FindStringSubmatch(href); len(match) > 1 {
				contacts.WhatsApp = cleanPhone(match[1])
			} else if strings.Contains(href, "whatsapp.com") || strings.Contains(href, "wa.me") {
				contacts.WhatsApp = extractPhoneFromAnyWA(href)
			}
		}

		// Email em mailto:
		if contacts.Email == "" && strings.HasPrefix(strings.ToLower(href), "mailto:") {
			email := strings.TrimPrefix(href, "mailto:")
			email = strings.Split(email, "?")[0]
			email = strings.TrimSpace(email)
			if isValidEmail(email) {
				contacts.Email = strings.ToLower(email)
			}
		}

		// Instagram em link
		if contacts.Instagram == "" && strings.Contains(href, "instagram.com") {
			if match := instaRegex.FindStringSubmatch(href); len(match) > 1 {
				user := match[1]
				if user != "p" && user != "explore" && user != "reels" && user != "stories" && user != "direct" {
					contacts.Instagram = "https://instagram.com/" + user
				}
			}
		}
	})

	// 2. Se WhatsApp ainda vazio, busca no texto do corpo
	htmlText := doc.Text()
	if contacts.WhatsApp == "" {
		if match := phoneBRRegex.FindString(htmlText); match != "" {
			contacts.WhatsApp = formatBRPhone(match)
		}
	}

	// 3. Se Email ainda vazio, busca por regex no texto
	if contacts.Email == "" {
		allEmails := emailRegex.FindAllString(htmlText, -1)
		for _, e := range allEmails {
			if isValidEmail(e) {
				contacts.Email = strings.ToLower(e)
				break
			}
		}
	}

	return contacts
}

// ScrapeBatch processa um lote de URLs concorrentemente usando um worker pool de até 10 goroutines
func (s *Scraper) ScrapeBatch(urls []string) []ExtractedContacts {
	results := make([]ExtractedContacts, len(urls))
	if len(urls) == 0 {
		return results
	}

	type job struct {
		index int
		url   string
	}

	type result struct {
		index    int
		contacts ExtractedContacts
	}

	jobs := make(chan job, len(urls))
	resChan := make(chan result, len(urls))

	// Pool limitado a no máximo 10 workers simultâneos
	workersCount := 10
	if len(urls) < workersCount {
		workersCount = len(urls)
	}

	var wg sync.WaitGroup
	for w := 0; w < workersCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				contacts := s.ScrapeLandingPage(j.url)
				resChan <- result{index: j.index, contacts: contacts}
			}
		}()
	}

	for i, u := range urls {
		jobs <- job{index: i, url: u}
	}
	close(jobs)

	wg.Wait()
	close(resChan)

	for r := range resChan {
		results[r.index] = r.contacts
	}

	return results
}

func cleanPhone(raw string) string {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
	if strings.HasPrefix(digits, "55") && len(digits) >= 12 {
		return digits
	}
	if len(digits) >= 10 && len(digits) <= 11 {
		return "55" + digits
	}
	return digits
}

func extractPhoneFromAnyWA(link string) string {
	u, err := url.Parse(link)
	if err == nil {
		phone := u.Query().Get("phone")
		if phone != "" {
			return cleanPhone(phone)
		}
	}
	digits := regexp.MustCompile(`\d{10,14}`).FindString(link)
	if digits != "" {
		return cleanPhone(digits)
	}
	return ""
}

func formatBRPhone(raw string) string {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
	if len(digits) >= 10 {
		if strings.HasPrefix(digits, "55") {
			return digits
		}
		return "55" + digits
	}
	return ""
}

func isValidEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) < 6 || !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return false
	}
	// Filtra extensões de imagens ou assets que colidem com regex
	invalidSuffixes := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".css", ".js"}
	for _, suf := range invalidSuffixes {
		if strings.HasSuffix(email, suf) {
			return false
		}
	}
	// Filtra domínios genéricos de templates
	invalidDomains := []string{"wixpress.com", "sentry.io", "example.com", "domain.com", "email.com"}
	for _, dom := range invalidDomains {
		if strings.Contains(email, dom) {
			return false
		}
	}
	return true
}
