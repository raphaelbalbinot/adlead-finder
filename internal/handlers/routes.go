package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/raphaelbalbinot/adlead-finder/internal/ai"
	"github.com/raphaelbalbinot/adlead-finder/internal/db"
	"github.com/raphaelbalbinot/adlead-finder/internal/meta"
	"github.com/raphaelbalbinot/adlead-finder/internal/scraper"
	"github.com/raphaelbalbinot/adlead-finder/web"
)

// Server gerencia as dependências e roteamento HTTP
type Server struct {
	db         *db.Database
	metaClient *meta.Client
	aiClient   *ai.Client
	router     *chi.Mux
}

// SearchRequest representa os parâmetros enviados pelo frontend para iniciar uma busca
type SearchRequest struct {
	SearchTerms        string   `json:"search_terms"`
	Limit              int      `json:"limit"`
	AdDeliveryDateMin  string   `json:"ad_delivery_date_min"`
	PublisherPlatforms []string `json:"publisher_platforms"`
	OnlyWhatsApp       bool     `json:"only_whatsapp"`
	OnlyEmail          bool     `json:"only_email"`
	MinScore           int      `json:"min_score"`
}

// UpdateStatusRequest payload para alteração de status do lead
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// NewServer inicializa o roteador e todos os middlewares e rotas da aplicação
func NewServer(database *db.Database, metaClient *meta.Client, aiClient *ai.Client) *Server {
	r := chi.NewRouter()

	// Middlewares essenciais
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	// Configuração de CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	s := &Server{
		db:         database,
		metaClient: metaClient,
		aiClient:   aiClient,
		router:     r,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Rotas da API REST
	s.router.Route("/api", func(r chi.Router) {
		r.Post("/search", s.handleSearch)
		r.Get("/leads", s.handleGetLeads)
		r.Patch("/leads/{id}/status", s.handleUpdateStatus)
		r.Delete("/leads/{id}", s.handleDeleteLead)
		r.Get("/stats", s.handleGetStats)
		r.Get("/export/csv", s.handleExportCSV)
	})

	// Servir arquivos estáticos do frontend embutido
	fileSys, err := web.GetFileSystem()
	if err == nil {
		fileServer := http.FileServer(fileSys)
		s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fileServer.ServeHTTP(w, r)
		}))
	}
}

// GetRouter retorna o manipulador HTTP principal
func (s *Server) GetRouter() http.Handler {
	return s.router
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSONError(w, "Requisição inválida. Envie um JSON com 'search_terms'", http.StatusBadRequest)
		return
	}

	req.SearchTerms = strings.TrimSpace(req.SearchTerms)
	if req.SearchTerms == "" {
		s.sendJSONError(w, "O termo de busca ('search_terms') não pode estar vazio", http.StatusBadRequest)
		return
	}

	target := req.Limit
	if target <= 0 {
		target = 25
	}
	if target > 100 {
		target = 100
	}

	metaParams := meta.SearchParams{
		SearchTerms:        req.SearchTerms,
		Limit:              target,
		AdDeliveryDateMin:  req.AdDeliveryDateMin,
		PublisherPlatforms: req.PublisherPlatforms,
	}

	ctx := r.Context()
	scraperObj := scraper.NewScraper()

	seenPageIDs := make(map[string]bool)
	var processedLeads []db.Lead
	var mu sync.Mutex

	afterCursor := ""
	maxPages := 10 // Itera até 10 páginas para acumular a meta exata com os filtros ativos

	for page := 1; page <= maxPages && len(processedLeads) < target; page++ {
		// 1. Busca lote de anúncios da Meta
		companiesBatch, nextCursor, err := s.metaClient.FetchAdsPage(metaParams, afterCursor)
		if err != nil {
			if len(processedLeads) > 0 {
				break // Se já coletou parte dos leads, retorna o que tem
			}
			log.Printf("Erro na busca da Meta API: %v", err)
			s.sendJSONError(w, fmt.Sprintf("Erro ao consultar a Meta Ad Library: %v", err), http.StatusBadGateway)
			return
		}

		// Filtra empresas já vistas neste ciclo
		var uniqueCompanies []meta.AggregatedCompany
		for _, comp := range companiesBatch {
			if !seenPageIDs[comp.PageID] {
				seenPageIDs[comp.PageID] = true
				uniqueCompanies = append(uniqueCompanies, comp)
			}
		}

		if len(uniqueCompanies) == 0 && nextCursor == "" {
			break
		}

		if len(uniqueCompanies) > 0 {
			// 2. Raspagem concorrente de Landing Pages deste lote
			urls := make([]string, len(uniqueCompanies))
			for i, c := range uniqueCompanies {
				urls[i] = c.LandingPageURL
			}
			contactsList := scraperObj.ScrapeBatch(urls)

			// 3. Filtra apenas os que atendem aos requisitos de contato
			var candidateBatch []meta.AggregatedCompany
			var candidateContacts []scraper.ExtractedContacts

			for i, comp := range uniqueCompanies {
				contacts := contactsList[i]

				if req.OnlyWhatsApp && contacts.WhatsApp == "" {
					continue
				}
				if req.OnlyEmail && contacts.Email == "" {
					continue
				}

				candidateBatch = append(candidateBatch, comp)
				candidateContacts = append(candidateContacts, contacts)
			}

			// 4. Qualifica com Gemini AI e persiste no SQLite
			if len(candidateBatch) > 0 {
				var wg sync.WaitGroup
				for i, comp := range candidateBatch {
					wg.Add(1)
					go func(idx int, c meta.AggregatedCompany) {
						defer wg.Done()

						contacts := candidateContacts[idx]

						leadInput := ai.LeadInputData{
							CompanyName:        c.CompanyName,
							ActiveAdsCount:     c.ActiveAdsCount,
							AdCreativeSample:   c.AdCreativeSample,
							LandingPageURL:     c.LandingPageURL,
							ExtractedWhatsApp:  contacts.WhatsApp,
							ExtractedEmail:     contacts.Email,
							ExtractedInstagram: contacts.Instagram,
						}

						aiResult := s.aiClient.QualifyLead(ctx, leadInput)

						if req.MinScore > 0 && aiResult.QualityScore < req.MinScore {
							return
						}

						analysisFull := aiResult.AnalysisReason
						if aiResult.ClassificationReason != "" {
							analysisFull = fmt.Sprintf("[%s] %s | %s", aiResult.Classification, aiResult.ClassificationReason, aiResult.AnalysisReason)
						}

						leadEntity := &db.Lead{
							PageID:              c.PageID,
							CompanyName:         c.CompanyName,
							ActiveAdsCount:      c.ActiveAdsCount,
							AdCreativeSample:    c.AdCreativeSample,
							AdSnapshotURL:       c.AdSnapshotURL,
							LandingPageURL:      c.LandingPageURL,
							ExtractedWhatsApp:   contacts.WhatsApp,
							ExtractedEmail:      contacts.Email,
							ExtractedInstagram:  contacts.Instagram,
							AIQualityScore:      aiResult.QualityScore,
							AIClassification:    aiResult.Classification,
							AIAnalysisReason:    analysisFull,
							AIIcebreaker:        aiResult.IcebreakerParagraph,
							BusinessSegment:     aiResult.BusinessSegment,
							AdDeliveryStartTime: c.AdDeliveryStartTime,
							AdCreationTime:      c.AdCreationTime,
							DaysRunning:         c.DaysRunning,
							Status:              "Novo",
						}

						mu.Lock()
						if len(processedLeads) < target {
							savedLead, err := s.db.UpsertLead(leadEntity)
							if err == nil && savedLead != nil {
								processedLeads = append(processedLeads, *savedLead)
							}
						}
						mu.Unlock()
					}(i, comp)
				}
				wg.Wait()
			}
		}

		if len(processedLeads) >= target || nextCursor == "" {
			break
		}
		afterCursor = nextCursor
	}

	if processedLeads == nil {
		processedLeads = []db.Lead{}
	}

	s.sendJSON(w, map[string]interface{}{
		"message": fmt.Sprintf("%d leads minerados e qualificados com sucesso!", len(processedLeads)),
		"total":   len(processedLeads),
		"leads":   processedLeads,
	}, http.StatusOK)
}

func (s *Server) handleGetLeads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	minScore, _ := strconv.Atoi(q.Get("min_score"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	if limit <= 0 {
		limit = 100
	}

	filter := db.LeadFilter{
		Status:       q.Get("status"),
		Search:       q.Get("search"),
		OnlyWhatsApp: q.Get("only_whatsapp") == "true" || q.Get("only_whatsapp") == "1",
		OnlyEmail:    q.Get("only_email") == "true" || q.Get("only_email") == "1",
		MinScore:     minScore,
		Limit:        limit,
		Offset:       offset,
	}

	leads, total, err := s.db.GetLeads(filter)
	if err != nil {
		s.sendJSONError(w, fmt.Sprintf("Erro ao buscar leads: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, map[string]interface{}{
		"leads":  leads,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}, http.StatusOK)
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.sendJSONError(w, "ID de lead inválido", http.StatusBadRequest)
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendJSONError(w, "JSON inválido. Esperado {'status': 'Novo|Contatado|Descartado'}", http.StatusBadRequest)
		return
	}

	req.Status = strings.TrimSpace(req.Status)
	validStatuses := map[string]bool{"Novo": true, "Contatado": true, "Descartado": true}
	if !validStatuses[req.Status] {
		s.sendJSONError(w, "Status inválido. Use 'Novo', 'Contatado' ou 'Descartado'", http.StatusBadRequest)
		return
	}

	if err := s.db.UpdateStatus(id, req.Status); err != nil {
		s.sendJSONError(w, fmt.Sprintf("Erro ao atualizar status: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, map[string]interface{}{
		"message": "Status atualizado com sucesso",
		"id":      id,
		"status":  req.Status,
	}, http.StatusOK)
}

func (s *Server) handleDeleteLead(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.sendJSONError(w, "ID de lead inválido", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteLead(id); err != nil {
		s.sendJSONError(w, fmt.Sprintf("Erro ao excluir lead: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, map[string]interface{}{
		"message": "Lead excluído com sucesso",
		"id":      id,
	}, http.StatusOK)
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		s.sendJSONError(w, fmt.Sprintf("Erro ao obter estatísticas: %v", err), http.StatusInternalServerError)
		return
	}
	s.sendJSON(w, stats, http.StatusOK)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	leads, _, err := s.db.GetLeads(db.LeadFilter{Limit: 10000})
	if err != nil {
		s.sendJSONError(w, "Erro ao carregar leads para exportação", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=leads_%s.csv", time.Now().Format("2006-01-02_150405")))

	// BOM para suporte a acentos no Excel
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{
		"ID", "Empresa", "Área de Atuação", "Dias Rodando", "Início de Veiculação", "Anúncios Ativos", "WhatsApp", "E-mail", "Instagram",
		"Landing Page", "Score IA", "Classificação", "Status", "Abordagem Sugerida", "Criativo", "Data de Cadastro",
	}
	_ = writer.Write(header)

	for _, l := range leads {
		daysStr := "0 dias"
		if l.DaysRunning > 0 {
			daysStr = fmt.Sprintf("%d dia(s)", l.DaysRunning)
		}
		row := []string{
			fmt.Sprintf("%d", l.ID),
			l.CompanyName,
			l.BusinessSegment,
			daysStr,
			l.AdDeliveryStartTime,
			fmt.Sprintf("%d", l.ActiveAdsCount),
			l.ExtractedWhatsApp,
			l.ExtractedEmail,
			l.ExtractedInstagram,
			l.LandingPageURL,
			fmt.Sprintf("%d", l.AIQualityScore),
			l.AIClassification,
			l.Status,
			l.AIIcebreaker,
			l.AdCreativeSample,
			l.CreatedAt.Format("02/01/2006 15:04"),
		}
		_ = writer.Write(row)
	}
}

func (s *Server) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) sendJSONError(w http.ResponseWriter, msg string, statusCode int) {
	s.sendJSON(w, map[string]interface{}{
		"error":   true,
		"message": msg,
		}, statusCode)
}
