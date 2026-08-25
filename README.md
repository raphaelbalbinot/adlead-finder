# AdLead Finder 🚀

Ferramenta em Go para busca, enriquecimento e qualificação de leads comerciais.

## 🛠️ Tecnologias
- **Go** (Golang)
- **VS Code** com suporte oficial para Go e gopls

## 🚀 Como Executar

### Pré-requisitos
- [Go 1.22+](https://go.dev/dl/) instalado e configurado no PATH.

### Executar a aplicação
```bash
go run ./cmd/api
```

### Compilar binário
```bash
go build -o bin/adlead-finder.exe ./cmd/api
```

### Executar testes
```bash
go test -v ./...
```

## 📂 Estrutura do Projeto
```text
adlead-finder/
├── cmd/
│   └── api/
│       └── main.go       # Ponto de entrada principal
├── internal/             # Lógica interna e regras de negócio
├── pkg/                  # Pacotes reutilizáveis
├── .vscode/              # Extensões e configurações recomendadas
├── .gitignore
├── go.mod
└── README.md
```
