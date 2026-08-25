# Implementation Plan: AdLead Finder (Golang Edition)

## Overview
Aplicação desktop/local completa em Go (Golang) com frontend SPA embutido (`//go:embed`), persistência física em SQLite puro (`modernc.org/sqlite` sem CGO), cliente de consulta à Meta Ad Library API, scraper concorrente com worker pool de landing pages e qualificador de leads B2B usando Google Gemini AI Studio (saída estruturada JSON).

## Architecture Decisions
1. **SQLite sem CGO (`modernc.org/sqlite`):** Garante compatibilidade nativa de compilação no Windows sem necessidade de GCC/MinGW.
2. **Chi Router (`github.com/go-chi/chi/v5`):** Roteador leve, idiomático e com excelente suporte a middleware e sub-roteamento.
3. **Frontend Embutido (`web/embed.go`):** Uso de `embed.FS` para encapsular HTML/CSS/JS dentro do executável `.exe`, simplificando distribuição.
4. **Resiliência e Tolerância a Falhas:** Timeout de 8s e User-Agent realista no scraper com tratamento individual por lead para garantir que falhas de scraping de uma página não afetem os demais leads.
5. **Fallback Gracioso da IA:** Fallback defensivo caso a chave de API não esteja configurada ou atinja quota, gerando pontuação e classificação heurística sem travar a aplicação.

## Task List
Veja tarefas detalhadas em [tasks/todo.md](file:///c:/Users/s023319089/Documents/Projetos/adlead-finder/tasks/todo.md).

## Risks and Mitigations
| Risco | Impacto | Mitigação |
| :--- | :--- | :--- |
| Landing pages lentas ou fora do ar | Médio | Worker pool com timeout de 8s por contexto + captura de erro isolada |
| Quota ou limite de taxa do Gemini | Médio | Fallback defensivo com heurística contextual de criativos |
| Meta API Token expirado ou inválido | Médio | Retorno amigável no endpoint com mensagem clara para o frontend |
