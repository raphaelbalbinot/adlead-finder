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
	waLinkRegex = regexp.MustCompile(`(?i)(?:wa\.me\/|api\.whatsapp\.com\/send\?phone=)(\+?\d{10,15})`)
	
	// Regex estrito para celulares do Brasil no texto HTML (DDD + 9XXXX-XXXX)
	phoneBRRegex = regexp.MustCompile(`(?:\+?55\s?)?(?:\(?([1-9]{2})\)?\s?)(?:9\s?\d{4}[-\s]?\d{4})`)
	
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

	// 1. Varrer links (href, onclick, data-whatsapp, data-href)
	doc.Find("a, button, [onclick], [data-whatsapp], [data-href]").Each(func(i int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		onclick, _ := sel.Attr("onclick")
		dataWA, _ := sel.Attr("data-whatsapp")
		dataHref, _ := sel.Attr("data-href")

		combined := strings.Join([]string{href, onclick, dataWA, dataHref}, " ")

		// WhatsApp em link ou atributo
		if contacts.WhatsApp == "" {
			if strings.Contains(combined, "whatsapp.com") || strings.Contains(combined, "wa.me") || strings.Contains(combined, "wa.link") || strings.Contains(combined, "tel:") {
				contacts.WhatsApp = extractPhoneFromAnyWA(combined)
			}
		}

		// Email em mailto:
		if contacts.Email == "" && strings.Contains(strings.ToLower(combined), "mailto:") {
			parts := strings.Split(combined, "mailto:")
			if len(parts) > 1 {
				email := strings.Split(parts[1], "?")[0]
				email = strings.Split(email, "\"")[0]
				email = strings.Split(email, "'")[0]
				email = strings.TrimSpace(email)
				if isValidEmail(email) {
					contacts.Email = strings.ToLower(email)
				}
			}
		}

		// Instagram em link
		if contacts.Instagram == "" && strings.Contains(combined, "instagram.com") {
			if match := instaRegex.FindStringSubmatch(combined); len(match) > 1 {
				user := match[1]
				if user != "p" && user != "explore" && user != "reels" && user != "stories" && user != "direct" {
					contacts.Instagram = "https://instagram.com/" + user
				}
			}
		}
	})

	// 2. Remove tags não-visíveis (scripts, estilos, SVG, iframes) antes de ler o texto
	doc.Find("script, style, noscript, svg, iframe, link, meta").Remove()
	htmlText := doc.Text()

	// 3. Se WhatsApp ainda vazio, busca por padrão estrito de celular no texto
	if contacts.WhatsApp == "" {
		allPhoneMatches := phoneBRRegex.FindAllString(htmlText, -1)
		for _, m := range allPhoneMatches {
			if valid := cleanAndValidateBRPhone(m); valid != "" {
				contacts.WhatsApp = valid
				break
			}
		}
	}

	// 4. Se Email ainda vazio, busca por regex no texto limpo
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

var validDDDs = map[string]bool{
	"11": true, "12": true, "13": true, "14": true, "15": true, "16": true, "17": true, "18": true, "19": true,
	"21": true, "22": true, "24": true, "27": true, "28": true,
	"31": true, "32": true, "33": true, "34": true, "35": true, "37": true, "38": true,
	"41": true, "42": true, "43": true, "44": true, "45": true, "46": true, "47": true, "48": true, "49": true,
	"51": true, "53": true, "54": true, "55": true,
	"61": true, "62": true, "63": true, "64": true, "65": true, "66": true, "67": true, "68": true, "69": true,
	"71": true, "73": true, "74": true, "75": true, "77": true, "79": true,
	"81": true, "82": true, "83": true, "84": true, "85": true, "86": true, "87": true, "88": true, "89": true,
	"91": true, "92": true, "93": true, "94": true, "95": true, "96": true, "97": true, "98": true, "99": true,
}

// cleanAndValidateBRPhone valida e normaliza número brasileiro de WhatsApp para formato 55 + DDD + 9 + 8 dígitos
func cleanAndValidateBRPhone(raw string) string {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
	
	// Remove leading zero se houver (ex: 011999999999 -> 11999999999)
	for strings.HasPrefix(digits, "0") {
		digits = digits[1:]
	}

	// Remove 55 se presente no início para validar o DDD nacional
	national := digits
	if strings.HasPrefix(national, "55") && len(national) >= 12 {
		national = national[2:]
	}

	// Valida comprimento: deve ter 10 (DDD + 8 dígitos) ou 11 (DDD + 9 dígitos)
	if len(national) != 10 && len(national) != 11 {
		return ""
	}

	ddd := national[:2]
	if !validDDDs[ddd] {
		return ""
	}

	number := national[2:]

	// Descartar números corporativos 0800, 4004, etc.
	if strings.HasPrefix(number, "0800") || strings.HasPrefix(number, "4004") || strings.HasPrefix(number, "3003") {
		return ""
	}

	// Descartar sequências repetidas ou placeholders (ex: 999999999, 123456789)
	if isDummyPhoneNumber(number) {
		return ""
	}

	// Se tem 11 dígitos: o nono dígito DEVE ser 9
	if len(national) == 11 {
		if !strings.HasPrefix(number, "9") {
			return "" // 11 dígitos que não começam com 9 no Brasil é inválido
		}
		return "55" + national
	}

	// Se tem 10 dígitos (DDD + 8 dígitos):
	// Se começa com [6-9], era celular no formato antigo -> adiciona o nono dígito 9 obrigatório
	if number[0] >= '6' && number[0] <= '9' {
		return "55" + ddd + "9" + number
	}

	// Se começa com [2-5], é telefone fixo (descartar para WhatsApp)
	return ""
}

func extractPhoneFromAnyWA(link string) string {
	link = strings.TrimSpace(link)
	if strings.Contains(link, "chat.whatsapp.com") || strings.Contains(link, "/message/") {
		return "" // Ignora links de grupos ou hashes de catálogo
	}

	u, err := url.Parse(link)
	if err == nil {
		phone := u.Query().Get("phone")
		if phone != "" {
			if valid := cleanAndValidateBRPhone(phone); valid != "" {
				return valid
			}
		}
	}

	// Regex específico para wa.me/5511... ou api.whatsapp.com/send?phone=...
	match := regexp.MustCompile(`(?:wa\.me\/|send\?phone=|\/p\/)(\+?\d{10,15})`).FindStringSubmatch(link)
	if len(match) > 1 {
		if valid := cleanAndValidateBRPhone(match[1]); valid != "" {
			return valid
		}
	}

	return ""
}

func formatBRPhone(raw string) string {
	return cleanAndValidateBRPhone(raw)
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

func isDummyPhoneNumber(num string) bool {
	if len(num) < 8 {
		return true
	}
	allSame := true
	for i := 1; i < len(num); i++ {
		if num[i] != num[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return true
	}
	if num == "999999999" || num == "123456789" || num == "987654321" || num == "900000000" || num == "988888888" {
		return true
	}
	return false
}
