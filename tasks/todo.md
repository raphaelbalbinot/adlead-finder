# Tasks: AdLead Finder (Golang Edition)

## Phase 1: Fundação e Dependências
- [x] Task 1.1: Instalar dependências Go (`chi`, `modernc.org/sqlite`, `goquery`, `cors`) e configurar `internal/config/config.go`
- [x] Task 1.2: Implementar módulo de persistência SQLite pura (`internal/db/sqlite.go`) com schema de leads e CRUD completo
- [x] Checkpoint 1: Módulo DB compilado e testado com SQLite em memória/físico

## Phase 2: Integrações e Núcleo de Processamento (Meta, Scraper, Gemini AI)
- [x] Task 2.1: Implementar cliente Meta Ad Library API (`internal/meta/client.go`) com agrupamento por `page_id`
- [x] Task 2.2: Implementar Scraper concorrente com worker pool (`internal/scraper/scraper.go`) para WhatsApp, emails e Instagram
- [x] Task 2.3: Implementar cliente de qualificação Gemini AI (`internal/ai/gemini.go`) com saída estruturada JSON
- [x] Checkpoint 2: Pipeline de busca -> scraping -> IA testada

## Phase 3: Handlers HTTP, API REST e Embed
- [x] Task 3.1: Implementar Handlers e Rotas da API (`internal/handlers/routes.go`) e ponto de entrada (`cmd/server/main.go`)
- [x] Task 3.2: Configurar `web/embed.go` com `//go:embed static/*`
- [x] Checkpoint 3: Servidor HTTP rodando e respondendo endpoints REST

## Phase 4: Frontend Dashboard Embutido (Tailwind + Alpine.js)
- [x] Task 4.1: Desenvolver interface completa (`web/static/index.html`, `style.css`) com tema escuro moderno e responsivo
- [x] Task 4.2: Implementar lógica reativa do frontend (`web/static/app.js`) com busca, filtros, cópia de copy e atualização de status
- [x] Checkpoint 4: Frontend comunicando perfeitamente com a API no binário

## Phase 5: Scripts de Automação Windows e Validação
- [x] Task 5.1: Criar `build.bat` e `start.bat` para Windows
- [x] Task 5.2: Compilar `adlead-finder.exe` e validar inicialização e persistência no `leads.db`
- [x] Checkpoint 5: Sistema validado e sincronizado no repositório GitHub
