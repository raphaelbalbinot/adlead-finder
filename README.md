<div align="center">

<img src="web/static/logo.png" alt="Nexus AdLead Finder Logo" width="120" height="120" style="border-radius: 24px; box-shadow: 0 0 25px rgba(0, 255, 170, 0.35);" />

# Nexus AdLead Finder 🚀
**Prospecção B2B, Scraping Concorrente & Qualificação Inteligente com IA**

*Parte do ecossistema de soluções de inteligência comercial **Nexus** (ao lado do NexusVBO).*

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![SQLite](https://img.shields.io/badge/Database-SQLite%20(Pure%20Go)-003B57?style=flat&logo=sqlite)](https://modernc.org/sqlite)
[![Gemini AI](https://img.shields.io/badge/AI-Google%20Gemini%201.5%20Flash-4285F4?style=flat&logo=google)](https://ai.google.dev/)
[![Meta Graph API](https://img.shields.io/badge/Meta%20Ads-Graph%20API%20v21.0-0668E1?style=flat&logo=meta)](https://developers.facebook.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

</div>

---

## 📖 Visão Geral

O **Nexus AdLead Finder** é uma aplicação desktop/local de alta performance desenvolvida em **Go (Golang)** voltada para agências, consultorias e times comerciais B2B. Ele extrai anunciantes ativos diretamente da **Meta Ad Library API**, realiza a raspagem concorrente de Landing Pages para identificar canais de contato e qualifica a maturidade comercial de cada empresa utilizando o **Google Gemini AI (Google AI Studio)**.

Todo o frontend SPA moderno (Tailwind CSS + Alpine.js) é embutido diretamente no binário final compilado (`//go:embed`), permitindo que toda a solução seja distribuída e executada como um único arquivo `.exe` autônomo, sem dependências externas de Node.js, CGO ou bancos de dados pesados.

---

## 📸 Funcionalidades Principais

* 🔍 **Mineração na Meta Ad Library:** Busca em tempo real por nicho ou palavra-chave com filtros avançados:
  * **Data Mínima de Veiculação (`ad_delivery_date_min`):** Filtra apenas empresas anunciando a partir de uma data específica.
  * **Plataformas de Anúncio (`publisher_platforms`):** Filtro seletivo por Facebook, Instagram, Messenger ou Audience Network.
* ⚡ **Scraping Concorrente com Worker Pool:** Pool de 10 goroutines simultâneas com timeout individual (8s) para extração automática de:
  * **WhatsApp:** Links `wa.me/`, APIs e padrões de telefones brasileiros normalizados.
  * **E-mails:** Tags `mailto:` e varredura textual no DOM com limpeza de falsos positivos.
  * **Instagram:** Perfis sociais vinculados.
* 🧠 **Qualificação Avançada com Gemini AI:** Diagnóstico estruturado em JSON com:
  * **Score de Qualidade (1 a 10):** Avaliação de investimento e maturidade digital.
  * **Classificação:** *Alto Potencial*, *Médio* ou *Descartável*.
  * **Diagnóstico & Justificativa:** Explicação transparente do porquê da classificação atribuída.
  * **Copy de Abordagem Personalizada (Icebreaker):** Mensagem de primeiro contato consultiva citando temas do anúncio.
* 🌓 **Alternador de Color-Mode (Claro / Escuro / Sistema):** Interface responsiva e adaptável com persistência da preferência e sincronização automática com o sistema operacional.
* 💾 **Persistência SQLite 100% Pura em Go:** Utiliza o driver `modernc.org/sqlite` sem necessidade de CGO/MinGW no Windows, salvando a base local em `./leads.db`.
* 🎯 **Ações Rápidas de 1-Clique:**
  * 🟢 **WhatsApp 1-Clique:** Abre conversa direta com a mensagem de abordagem da IA pré-preenchida.
  * ✉️ **E-mail 1-Clique:** Dispara o cliente de e-mail padrão (`mailto:`) com assunto e corpo configurados.
  * 📋 **Copiar Copy:** Cópia imediata da mensagem para a área de transferência.
  * 🔄 **Gestão de Status:** Alterne entre *Novo*, *Contatado* e *Descartado*.
  * 📊 **Exportar CSV:** Download da base formatada com suporte a acentos no Excel.

---

## 🛠️ Stack Tecnológica

| Camada | Tecnologia | Detalhes |
| :--- | :--- | :--- |
| **Linguagem & Runtime** | Go (Golang) 1.22+ | Compilação nativa para Windows sem dependências |
| **Roteador HTTP** | `github.com/go-chi/chi/v5` | Roteador leve, idiomático com middleware de timeout e CORS |
| **Banco de Dados** | `modernc.org/sqlite` | Driver SQLite 100% puro em Go (sem CGO) |
| **Scraping & DOM** | `github.com/PuerkitoBio/goquery` | Seletores CSS para extração ágil de tags e links |
| **Inteligência Artificial**| Google Gemini 1.5 Flash | Chamadas HTTP REST com retorno de JSON estruturado |
| **Frontend Embutido** | Tailwind CSS + Alpine.js | Dashboard SPA empacotado via `//go:embed static/*` |
| **Família Nexus** | Padrão NexusVBO | Identidade visual alinhada às soluções de inteligência Nexus |

---

## 📂 Estrutura de Diretórios

```text
adlead-finder/
├── cmd/
│   └── server/
│       └── main.go           # Ponto de entrada, servidor HTTP e graceful shutdown
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
│       ├── logo.png          # Logotipo oficial Nexus
│       ├── favicon.png       # Favicon da aplicação
│       ├── logo.svg          # Logotipo vetorial de alta definição
│       ├── index.html        # Dashboard SPA completo
│       ├── app.js            # Lógica reativa com Alpine.js e Color Mode
│       └── style.css         # Estilos glassmorphism, temas claro e escuro
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
Copie o modelo de exemplo ou edite o seu arquivo `.env`:

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

---

## 📜 Licença

Este projeto é distribuído sob a licença **MIT**.
