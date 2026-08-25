package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LeadInputData contém as informações contextuais do lead para análise da IA
type LeadInputData struct {
	CompanyName        string `json:"company_name"`
	ActiveAdsCount     int    `json:"active_ads_count"`
	AdCreativeSample   string `json:"ad_creative_sample"`
	LandingPageURL     string `json:"landing_page_url"`
	ExtractedWhatsApp  string `json:"extracted_whatsapp"`
	ExtractedEmail     string `json:"extracted_email"`
	ExtractedInstagram string `json:"extracted_instagram"`
}

// QualificationResult representa a análise estruturada retornada pelo Gemini
type QualificationResult struct {
	QualityScore         int    `json:"quality_score"`
	Classification       string `json:"classification"`        // "Alto Potencial", "Médio", "Descartável"
	ClassificationReason string `json:"classification_reason"` // Explicação detalhada do porquê da classificação
	AnalysisReason       string `json:"analysis_reason"`       // Análise contextual completa
	IcebreakerParagraph  string `json:"icebreaker_paragraph"`  // Copy de abordagem comercial 1-clique
	BusinessSegment      string `json:"business_segment"`      // Área/nicho específico de atuação da empresa
}

// Client gerencia a comunicação com o Google AI Studio
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient cria uma nova instância do cliente Gemini
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
	}
}

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMIMEType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// QualifyLead analisa o lead utilizando o modelo Gemini 3.7 Flash
func (c *Client) QualifyLead(ctx context.Context, data LeadInputData) QualificationResult {
	if c.apiKey == "" {
		return c.fallbackQualification(data, "Chave GEMINI_API_KEY não configurada no .env")
	}

	prompt := fmt.Sprintf(`Você é um especialista sênior em Prospecção B2B e Qualificação de Leads de Tráfego Pago.
Analise os dados deste anunciante extraído da Meta Ads Library e avalie sua maturidade comercial.

DADOS DO LEAD:
- Empresa: %s
- Quantidade de Anúncios Ativos: %d
- Amostra do Criativo: "%s"
- Landing Page: %s
- WhatsApp Encontrado: %s
- E-mail Encontrado: %s
- Instagram: %s

INSTRUÇÕES OBRIGATÓRIAS:
1. Atribua uma nota 'quality_score' de 1 a 10 (Considere: múltiplos criativos ativos = orçamento real; ter WhatsApp/Landing Page = funil estruturado).
2. Defina 'classification': "Alto Potencial" (score >= 8), "Médio" (score 5 a 7) ou "Descartável" (score < 5).
3. Escreva 'classification_reason' explicando claramente por que esta classificação foi dada com base nos dados.
4. Escreva 'analysis_reason' sintetizando a maturidade do anunciante.
5. Crie um 'icebreaker_paragraph' altamente persuasivo e personalizado em português do Brasil, mencionando o tema do anúncio deles e iniciando uma conversa consultiva sem ser agressivo.
6. Identifique em 'business_segment' a área/nicho de atuação específica e precisa da empresa anunciante (ex: "Clínica Odontológica e Implantes", "Energia Solar Fotovoltaica B2B", "Advocacia Trabalhista e Previdenciária", "Consultoria Financeira", etc.).

Retorne EXCLUSIVAMENTE um objeto JSON válido com a seguinte estrutura:
{
  "quality_score": 8,
  "classification": "Alto Potencial",
  "classification_reason": "Empresa veicula múltiplos anúncios com direcionamento para WhatsApp validado, demonstrando investimento contínuo e processo comercial estruturado.",
  "analysis_reason": "Empresa roda criativos ativos com tráfego direcionado, demonstrando orçamento e validação comercial.",
  "icebreaker_paragraph": "Olá equipe da [Nome], vi que vocês estão veiculando campanhas no Instagram sobre [Tema]...",
  "business_segment": "Energia Solar Fotovoltaica"
}`,
		data.CompanyName, data.ActiveAdsCount, data.AdCreativeSample, data.LandingPageURL,
		data.ExtractedWhatsApp, data.ExtractedEmail, data.ExtractedInstagram,
	)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:      0.2,
			ResponseMIMEType: "application/json",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return c.fallbackQualification(data, "Erro interno ao serializar payload Gemini")
	}

	modelsToTry := []string{"gemini-3.7-flash", "gemini-3.1-flash-lite", "gemini-3.5-flash-lite", "gemini-3.5-flash"}
	var lastErr string

	for _, modelName := range modelsToTry {
		endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, c.apiKey)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Sprintf("Erro de conexão com %s: %v", modelName, err)
			continue
		}

		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Sprintf("Erro ao ler resposta de %s: %v", modelName, err)
			continue
		}

		var gResp geminiResponse
		if err := json.Unmarshal(respBytes, &gResp); err != nil {
			lastErr = fmt.Sprintf("Erro ao decodificar JSON de %s", modelName)
			continue
		}

		if gResp.Error != nil {
			lastErr = fmt.Sprintf("%s: %s", modelName, gResp.Error.Message)
			continue
		}

		if len(gResp.Candidates) > 0 && len(gResp.Candidates[0].Content.Parts) > 0 {
			jsonText := gResp.Candidates[0].Content.Parts[0].Text
			jsonText = cleanMarkdownJSON(jsonText)

			var result QualificationResult
			if err := json.Unmarshal([]byte(jsonText), &result); err == nil {
				// Normalizações de segurança
				if result.QualityScore < 1 {
					result.QualityScore = 1
				} else if result.QualityScore > 10 {
					result.QualityScore = 10
				}

				if result.Classification == "" {
					if result.QualityScore >= 8 {
						result.Classification = "Alto Potencial"
					} else if result.QualityScore >= 5 {
						result.Classification = "Médio"
					} else {
						result.Classification = "Descartável"
					}
				}

				return result
			}
		}
	}

	return c.fallbackQualification(data, lastErr)
}

func (c *Client) fallbackQualification(data LeadInputData, reason string) QualificationResult {
	score := 5
	classification := "Médio"

	if data.ActiveAdsCount >= 3 && data.ExtractedWhatsApp != "" {
		score = 8
		classification = "Alto Potencial"
	} else if data.ActiveAdsCount >= 2 || data.ExtractedEmail != "" || data.LandingPageURL != "" {
		score = 6
		classification = "Médio"
	} else {
		score = 4
		classification = "Descartável"
	}

	classReason := fmt.Sprintf("Classificação heurística: Anunciante possui %d anúncio(s) ativo(s) e presença digital detectada.", data.ActiveAdsCount)
	if reason != "" {
		classReason += fmt.Sprintf(" (Nota: %s)", reason)
	}

	icebreaker := fmt.Sprintf(
		"Olá equipe da %s, notei que vocês estão com campanhas ativas no Meta Ads. Como está o retorno e a qualificação dos leads que chegam para vocês atualmente?",
		data.CompanyName,
	)

	return QualificationResult{
		QualityScore:         score,
		Classification:       classification,
		ClassificationReason: classReason,
		AnalysisReason:       fmt.Sprintf("Empresa com %d anúncio(s) veiculando. Contatos: WhatsApp (%s), E-mail (%s).", data.ActiveAdsCount, data.ExtractedWhatsApp, data.ExtractedEmail),
		IcebreakerParagraph:  icebreaker,
		BusinessSegment:      "Tráfego Pago & Serviços",
	}
}

func cleanMarkdownJSON(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}
