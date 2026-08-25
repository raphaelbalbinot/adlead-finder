package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Lead representa a entidade principal de um lead minerado e qualificado
type Lead struct {
	ID                  int64     `json:"id"`
	PageID              string    `json:"page_id"`
	CompanyName         string    `json:"company_name"`
	ActiveAdsCount      int       `json:"active_ads_count"`
	AdCreativeSample    string    `json:"ad_creative_sample"`
	AdSnapshotURL       string    `json:"ad_snapshot_url"`
	LandingPageURL      string    `json:"landing_page_url"`
	ExtractedWhatsApp   string    `json:"extracted_whatsapp"`
	ExtractedEmail      string    `json:"extracted_email"`
	ExtractedInstagram  string    `json:"extracted_instagram"`
	AIQualityScore      int       `json:"ai_quality_score"`
	AIClassification    string    `json:"ai_classification"`
	AIAnalysisReason    string    `json:"ai_analysis_reason"`
	AIIcebreaker        string    `json:"ai_icebreaker"`
	BusinessSegment     string    `json:"business_segment"` // Área/nicho de atuação da empresa
	AdDeliveryStartTime string    `json:"ad_delivery_start_time"`
	AdCreationTime      string    `json:"ad_creation_time"`
	DaysRunning         int       `json:"days_running"`
	Status              string    `json:"status"` // "Novo", "Contatado", "Descartado"
	CreatedAt           time.Time `json:"created_at"`
}

// Stats armazena contadores e métricas de desempenho dos leads
type Stats struct {
	TotalLeads         int `json:"total_leads"`
	NewLeads           int `json:"new_leads"`
	ContactedLeads     int `json:"contacted_leads"`
	DiscardedLeads     int `json:"discarded_leads"`
	HighPotentialLeads int `json:"high_potential_leads"`
	WithWhatsApp       int `json:"with_whatsapp"`
	WithEmail          int `json:"with_email"`
}

// Database gerencia a conexão e operações no SQLite
type Database struct {
	conn *sql.DB
}

// InitDB inicializa o banco SQLite em ./leads.db e aplica as migrações
func InitDB(dbPath string) (*Database, error) {
	if dbPath == "" {
		dbPath = "./leads.db"
	}

	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir banco sqlite: %w", err)
	}

	// Limita conexões para evitar locks no SQLite
	conn.SetMaxOpenConns(1)

	db := &Database{conn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("falha ao aplicar migrações: %w", err)
	}

	log.Printf("✓ Banco de dados SQLite inicializado com sucesso em %s", dbPath)
	return db, nil
}

func (d *Database) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS leads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		page_id TEXT UNIQUE NOT NULL,
		company_name TEXT NOT NULL,
		active_ads_count INTEGER DEFAULT 1,
		ad_creative_sample TEXT,
		ad_snapshot_url TEXT,
		landing_page_url TEXT,
		extracted_whatsapp TEXT,
		extracted_email TEXT,
		extracted_instagram TEXT,
		ai_quality_score INTEGER DEFAULT 0,
		ai_classification TEXT DEFAULT 'Médio',
		ai_analysis_reason TEXT,
		ai_icebreaker TEXT,
		business_segment TEXT DEFAULT '',
		ad_delivery_start_time TEXT DEFAULT '',
		ad_creation_time TEXT DEFAULT '',
		days_running INTEGER DEFAULT 0,
		status TEXT DEFAULT 'Novo',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_leads_page_id ON leads(page_id);
	CREATE INDEX IF NOT EXISTS idx_leads_status ON leads(status);
	CREATE INDEX IF NOT EXISTS idx_leads_created_at ON leads(created_at);
	`
	if _, err := d.conn.Exec(schema); err != nil {
		return err
	}

	// Migração retroativa de colunas se a tabela já existir de execuções anteriores
	_, _ = d.conn.Exec("ALTER TABLE leads ADD COLUMN business_segment TEXT DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE leads ADD COLUMN ad_delivery_start_time TEXT DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE leads ADD COLUMN ad_creation_time TEXT DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE leads ADD COLUMN days_running INTEGER DEFAULT 0;")
	return nil
}

// Close fecha a conexão com o banco
func (d *Database) Close() error {
	return d.conn.Close()
}

// UpsertLead insere um novo lead ou atualiza dados relevantes se a page_id já existir
func (d *Database) UpsertLead(lead *Lead) (*Lead, error) {
	query := `
	INSERT INTO leads (
		page_id, company_name, active_ads_count, ad_creative_sample, ad_snapshot_url,
		landing_page_url, extracted_whatsapp, extracted_email, extracted_instagram,
		ai_quality_score, ai_classification, ai_analysis_reason, ai_icebreaker, business_segment,
		ad_delivery_start_time, ad_creation_time, days_running, status
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'Novo'))
	ON CONFLICT(page_id) DO UPDATE SET
		company_name = excluded.company_name,
		active_ads_count = excluded.active_ads_count,
		ad_creative_sample = excluded.ad_creative_sample,
		ad_snapshot_url = excluded.ad_snapshot_url,
		landing_page_url = excluded.landing_page_url,
		extracted_whatsapp = CASE WHEN excluded.extracted_whatsapp != '' THEN excluded.extracted_whatsapp ELSE leads.extracted_whatsapp END,
		extracted_email = CASE WHEN excluded.extracted_email != '' THEN excluded.extracted_email ELSE leads.extracted_email END,
		extracted_instagram = CASE WHEN excluded.extracted_instagram != '' THEN excluded.extracted_instagram ELSE leads.extracted_instagram END,
		ai_quality_score = excluded.ai_quality_score,
		ai_classification = excluded.ai_classification,
		ai_analysis_reason = excluded.ai_analysis_reason,
		ai_icebreaker = excluded.ai_icebreaker,
		business_segment = CASE WHEN excluded.business_segment != '' THEN excluded.business_segment ELSE leads.business_segment END,
		ad_delivery_start_time = CASE WHEN excluded.ad_delivery_start_time != '' THEN excluded.ad_delivery_start_time ELSE leads.ad_delivery_start_time END,
		ad_creation_time = CASE WHEN excluded.ad_creation_time != '' THEN excluded.ad_creation_time ELSE leads.ad_creation_time END,
		days_running = CASE WHEN excluded.days_running > 0 THEN excluded.days_running ELSE leads.days_running END
	RETURNING id, created_at;
	`

	err := d.conn.QueryRow(
		query,
		lead.PageID, lead.CompanyName, lead.ActiveAdsCount, lead.AdCreativeSample, lead.AdSnapshotURL,
		lead.LandingPageURL, lead.ExtractedWhatsApp, lead.ExtractedEmail, lead.ExtractedInstagram,
		lead.AIQualityScore, lead.AIClassification, lead.AIAnalysisReason, lead.AIIcebreaker, lead.BusinessSegment,
		lead.AdDeliveryStartTime, lead.AdCreationTime, lead.DaysRunning, lead.Status,
	).Scan(&lead.ID, &lead.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("falha ao salvar lead: %w", err)
	}

	return lead, nil
}

// LeadFilter encapsula os parâmetros de filtragem da listagem
type LeadFilter struct {
	Status       string
	Search       string
	OnlyWhatsApp bool
	OnlyEmail    bool
	MinScore     int
	Limit        int
	Offset       int
}

// GetLeads retorna a lista de leads com base nos filtros
func (d *Database) GetLeads(f LeadFilter) ([]Lead, int, error) {
	var whereClauses []string
	var args []interface{}

	if f.Status != "" && strings.ToLower(f.Status) != "todos" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, f.Status)
	}

	if f.Search != "" {
		searchTerm := "%" + strings.ToLower(f.Search) + "%"
		whereClauses = append(whereClauses, "(LOWER(company_name) LIKE ? OR LOWER(ad_creative_sample) LIKE ? OR LOWER(extracted_email) LIKE ? OR LOWER(extracted_whatsapp) LIKE ?)")
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
	}

	if f.OnlyWhatsApp {
		whereClauses = append(whereClauses, "extracted_whatsapp != ''")
	}

	if f.OnlyEmail {
		whereClauses = append(whereClauses, "extracted_email != ''")
	}

	if f.MinScore > 0 {
		whereClauses = append(whereClauses, "ai_quality_score >= ?")
		args = append(args, f.MinScore)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Contagem total
	countQuery := "SELECT COUNT(*) FROM leads" + whereSQL
	var total int
	if err := d.conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("falha ao contar leads: %w", err)
	}

	// Consulta de registros
	query := `
	SELECT id, page_id, company_name, active_ads_count, ad_creative_sample, ad_snapshot_url,
	       landing_page_url, extracted_whatsapp, extracted_email, extracted_instagram,
	       ai_quality_score, ai_classification, ai_analysis_reason, ai_icebreaker,
	       COALESCE(business_segment, ''), COALESCE(ad_delivery_start_time, ''),
	       COALESCE(ad_creation_time, ''), COALESCE(days_running, 0),
	       status, created_at
	FROM leads
	` + whereSQL + " ORDER BY id DESC"

	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
		if f.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, f.Offset)
		}
	}

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("falha ao buscar leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		err := rows.Scan(
			&l.ID, &l.PageID, &l.CompanyName, &l.ActiveAdsCount, &l.AdCreativeSample, &l.AdSnapshotURL,
			&l.LandingPageURL, &l.ExtractedWhatsApp, &l.ExtractedEmail, &l.ExtractedInstagram,
			&l.AIQualityScore, &l.AIClassification, &l.AIAnalysisReason, &l.AIIcebreaker,
			&l.BusinessSegment, &l.AdDeliveryStartTime, &l.AdCreationTime, &l.DaysRunning,
			&l.Status, &l.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("falha ao ler linha de lead: %w", err)
		}
		leads = append(leads, l)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("erro durante iteração de leads: %w", err)
	}

	if leads == nil {
		leads = []Lead{}
	}

	return leads, total, nil
}

// GetLeadByID busca um lead pelo ID
func (d *Database) GetLeadByID(id int64) (*Lead, error) {
	query := `
	SELECT id, page_id, company_name, active_ads_count, ad_creative_sample, ad_snapshot_url,
	       landing_page_url, extracted_whatsapp, extracted_email, extracted_instagram,
	       ai_quality_score, ai_classification, ai_analysis_reason, ai_icebreaker,
	       COALESCE(business_segment, ''), COALESCE(ad_delivery_start_time, ''),
	       COALESCE(ad_creation_time, ''), COALESCE(days_running, 0),
	       status, created_at
	FROM leads WHERE id = ?
	`
	var l Lead
	err := d.conn.QueryRow(query, id).Scan(
		&l.ID, &l.PageID, &l.CompanyName, &l.ActiveAdsCount, &l.AdCreativeSample, &l.AdSnapshotURL,
		&l.LandingPageURL, &l.ExtractedWhatsApp, &l.ExtractedEmail, &l.ExtractedInstagram,
		&l.AIQualityScore, &l.AIClassification, &l.AIAnalysisReason, &l.AIIcebreaker,
		&l.BusinessSegment, &l.AdDeliveryStartTime, &l.AdCreationTime, &l.DaysRunning,
		&l.Status, &l.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// UpdateStatus atualiza o status de um lead
func (d *Database) UpdateStatus(id int64, status string) error {
	query := `UPDATE leads SET status = ? WHERE id = ?`
	res, err := d.conn.Exec(query, status, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("lead não encontrado com id %d", id)
	}
	return nil
}

// DeleteLead remove um lead do banco
func (d *Database) DeleteLead(id int64) error {
	query := `DELETE FROM leads WHERE id = ?`
	_, err := d.conn.Exec(query, id)
	return err
}

// GetStats calcula contadores gerais do banco
func (d *Database) GetStats() (*Stats, error) {
	query := `
	SELECT 
		COUNT(*) as total,
		SUM(CASE WHEN status = 'Novo' THEN 1 ELSE 0 END) as novos,
		SUM(CASE WHEN status = 'Contatado' THEN 1 ELSE 0 END) as contatados,
		SUM(CASE WHEN status = 'Descartado' THEN 1 ELSE 0 END) as descartados,
		SUM(CASE WHEN ai_quality_score >= 8 THEN 1 ELSE 0 END) as alto_potencial,
		SUM(CASE WHEN extracted_whatsapp != '' THEN 1 ELSE 0 END) as com_whatsapp,
		SUM(CASE WHEN extracted_email != '' THEN 1 ELSE 0 END) as com_email
	FROM leads
	`
	var stats Stats
	var total, novos, contatados, descartados, altoPotencial, comWhatsapp, comEmail sql.NullInt64

	err := d.conn.QueryRow(query).Scan(
		&total, &novos, &contatados, &descartados, &altoPotencial, &comWhatsapp, &comEmail,
	)
	if err != nil {
		return nil, err
	}

	stats.TotalLeads = int(total.Int64)
	stats.NewLeads = int(novos.Int64)
	stats.ContactedLeads = int(contatados.Int64)
	stats.DiscardedLeads = int(descartados.Int64)
	stats.HighPotentialLeads = int(altoPotencial.Int64)
	stats.WithWhatsApp = int(comWhatsapp.Int64)
	stats.WithEmail = int(comEmail.Int64)

	return &stats, nil
}
