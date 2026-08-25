# AdLead Finder (Golang Edition) 🚀

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![SQLite](https://img.shields.io/badge/Database-SQLite%20(Pure%20Go)-003B57?style=flat&logo=sqlite)](https://modernc.org/sqlite)
[![Gemini AI](https://img.shields.io/badge/AI-Google%20Gemini%201.5%20Flash-4285F4?style=flat&logo=google)](https://ai.google.dev/)
[![Meta Graph API](https://img.shields.io/badge/Meta%20Ads-Graph%20API%20v21.0-0668E1?style=flat&logo=meta)](https://developers.facebook.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Ferramenta local de alta performance em **Go (Golang)** para prospecção, raspagem concorrente de contatos e qualificação inteligente de leads B2B extraídos da **Meta Ad Library API**, enriquecidos com scraping de Landing Pages e avaliados pelo modelo **Google Gemini (Google AI Studio)**.

O frontend Single Page Application (Tailwind CSS + Alpine.js) é 100% embutido diretamente no binário compilado (`//go:embed`), permitindo que a aplicação rode como um único arquivo `.exe` sem dependências externas.

---

## 📸 Funcionalidades Principais

* 🔍 **Mineração na Meta Ad Library:** Busca anúncios ativos por nicho/palavra-chave, com filtros por data mínima de veiculação (`ad_delivery_date_min`) e plataformas de veiculação (`publisher_platforms`: Facebook, Instagram, Messenger, Audience Network).
* ⚡ **Scraping Concorrente com Worker Pool:** Pool de 10 goroutines simultâneas com timeout individual (8s) para raspar Landing Pages dos anunciantes sem travar o fluxo, extraindo:
  * **WhatsApp:** Links diretos (`wa.me/`, `api.whatsapp.com`) e padrões de telefones brasileiros.
  * **E-mails:** Tags `mailto:` e varredura textual no DOM (com filtros de falsos positivos).
  * **Instagram:** Perfis sociais vinculados.
* 🧠 **Qualificação com Google Gemini AI:** Análise contextual dos criativos e maturidade comercial via saída estruturada JSON:
  * **Quality Score:** Nota de 1 a 10.
  * **Classificação:** *Alto Potencial*, *Médio* ou *Descartável*.
  * **Diagnóstico & Justificativa:** Explicação detalhada dos motivos da pontuação.
  * **Copy de Abordagem Personalizada (Icebreaker):** Mensagem de primeiro contato consultiva e customizada.
* 💾 **Persistência SQLite 100% Pura em Go:** Utiliza o driver `modernc.org/sqlite` que dispensa compilador CGO/GCC no Windows e persiste os dados fisicamente em `./leads.db`.
* 🎯 **Ações de 1-Clique no Dashboard:**
  * 🟢 **WhatsApp 1-Clique:** Abre a conversa diretamente com a copy pré-preenchida no `wa.me`.
  * ✉️ **E-mail 1-Clique:** Abre o cliente de e-mail padrão (`mailto:`) com assunto e corpo configurados.
  * 📋 **Copiar Copy:** Copia a mensagem de abordagem da IA para a área de transferência.
  * 🔄 **Gestão de Pipeline:** Alternância ágil de status (*Novo*, *Contatado*, *Descartado*).
  * 📊 **Exportação CSV:** Download de toda a base em 1 clique com suporte a acentos no Excel.

---

## 🛠️ Stack Tecnológica

| Camada | Tecnologia | Descrição |
| :--- | :--- | :--- |
| **Backend** | Go (Golang) 1.22+ | Linguagem compilada de alto desempenho e concorrência nativa |
| **Roteador HTTP** | `github.com/go-chi/chi/v5` | Roteador REST leve, idiomático e com middlewares de timeout e CORS |
| **Banco de Dados** | `modernc.org/sqlite` | SQLite puro em Go (sem dependência de CGO/MinGW no Windows) |
| **Scraping & HTML** | `github.com/PuerkitoBio/goquery` | Parser HTML idiomático baseado em seletores CSS |
| **Inteligência Artificial**| Google Gemini 1.5 Flash | Chamadas HTTP REST com retorno de JSON Schema estruturado |
| **Frontend** | Tailwind CSS + Alpine.js | Dashboard SPA moderno com tema escuro e glassmorphism |
| **Empacotamento** | `//go:embed` nativo | Frontend embutido diretamente dentro do binário `.exe` |

---

## 📂 Estrutura do Projeto

```text
adlead-finder/
├── cmd/
│   └── server/
│       └── main.go           # Ponto de entrada, inicialização e graceful shutdown
│
├── internal/
│   ├── config/
│   │   └── config.go         # Carregamento de variáveis de ambiente (.env)
│   ├── db/
│   │   └── sqlite.go         # Conexão física leads.db, schema e operações de CRUD
│   ├── meta/
│   │   └── client.go         # Cliente HTTP para Meta Ad Library API
│   ├── scraper/
│   │   └── scraper.go        # Worker pool concorrente para Landing Pages
│   ├── ai/
│   │   └── gemini.go         # Qualificador Gemini AI com saída JSON e fallback
│   └── handlers/
│       ├── routes.go         # Endpoints REST e servidor de arquivos estáticos
│       └── scraper_bridge.go # Ponte de execução de scraping
│
├── web/
│   ├── embed.go              # Diretiva //go:embed static/*
│   └── static/
│       ├── index.html        # Dashboard SPA completo
│       ├── app.js            # Lógica reativa com Alpine.js
│       └── style.css         # Estilos glassmorphism e tema escuro
│
├── build.bat                 # Script de compilação Windows (.exe otimizado)
├── start.bat                 # Script de inicialização e abertura automática do navegador
├── .env.example              # Modelo público de variáveis de ambiente
├── .env                      # Arquivo local privado de chaves de API (ignorado no Git)
├── go.mod
└── README.md
```

---

## 🚀 Como Executar

### 1. Pré-requisitos
* [Go 1.22+](https://go.dev/dl/) instalado e disponível no PATH.
* Token de Acesso da **Meta Graph API** ([Meta for Developers](https://developers.facebook.com/)).
* Chave de API do **Google AI Studio / Gemini** ([Google AI Studio](https://aistudio.google.com/)).

### 2. Configurar Variáveis de Ambiente (`.env`)
Copie o modelo de exemplo ou edite seu arquivo `.env`:

```bash
cp .env.example .env
```

Preencha com suas credenciais:
```env
# Porta do Servidor Local
PORT=8080

# Chave de Acesso do Meta for Developers (Graph API)
META_ACCESS_TOKEN=seu_meta_token_aqui

# Chave de API do Google AI Studio (Gemini)
GEMINI_API_KEY=sua_chave_gemini_aqui

# Ambiente
ENV=development
```

> 🔒 **Segurança:** O arquivo `.env` e os arquivos de banco `*.db` são estritamente ignorados pelo `.gitignore` e nunca serão enviados para o repositório público.

---

### 3. Execução Rápida no Windows (Recomendado)

Dê um duplo clique no arquivo **`start.bat`** (ou execute no terminal):

```cmd
.\start.bat
```

O script compilará o binário (se ainda não existir), iniciará o serviço e abrirá o navegador automaticamente em:
👉 **`http://localhost:8080`**

---

### 4. Execução Manual via Go CLI

**Executar em modo de desenvolvimento:**
```bash
go run ./cmd/server/main.go
```

**Compilar binário de produção otimizado:**
```bash
go build -ldflags="-s -w" -o adlead-finder.exe ./cmd/server/main.go
```

---

## 🔌 Documentação da API REST

| Método | Endpoint | Descrição |
| :--- | :--- | :--- |
| `POST` | `/api/search` | Dispara busca na Meta, executa scraping concorrente, qualifica com Gemini AI e salva no SQLite |
| `GET` | `/api/leads` | Lista leads salvos com suporte a filtros (`status`, `search`, `only_whatsapp`, `only_email`, `min_score`, `limit`, `offset`) |
| `PATCH` | `/api/leads/{id}/status` | Atualiza o status de um lead (`Novo`, `Contatado`, `Descartado`) |
| `DELETE` | `/api/leads/{id}` | Exclui fisicamente um lead da base de dados |
| `GET` | `/api/stats` | Retorna métricas consolidadas de leads e contatos disponíveis |
| `GET` | `/api/export/csv` | Gera e faz o download de um arquivo CSV de toda a base |

### Exemplo de Payload para Busca (`POST /api/search`):
```json
{
  "search_terms": "energia solar",
  "limit": 25,
  "ad_delivery_date_min": "2026-01-01",
  "publisher_platforms": ["FACEBOOK", "INSTAGRAM"],
  "only_whatsapp": false,
  "only_email": false,
  "min_score": 0
}
```

---

## 📜 Licença

Este projeto é distribuído sob a licença **MIT**. Consulte o arquivo [LICENSE](LICENSE) para mais detalhes.
