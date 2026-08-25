package handlers

import (
	"github.com/raphaelbalbinot/adlead-finder/internal/scraper"
)

type internalScrapeItem struct {
	WhatsApp  string
	Email     string
	Instagram string
}

func runScraperBatchInternal(urls []string) []internalScrapeItem {
	sc := scraper.NewScraper()
	contactsList := sc.ScrapeBatch(urls)
	
	items := make([]internalScrapeItem, len(contactsList))
	for i, c := range contactsList {
		items[i] = internalScrapeItem{
			WhatsApp:  c.WhatsApp,
			Email:     c.Email,
			Instagram: c.Instagram,
		}
	}
	return items
}
