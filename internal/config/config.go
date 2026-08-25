package config

import (
	"bufio"
	"os"
	"strings"
)

// Config armazena as configurações do ambiente da aplicação
type Config struct {
	Port            string
	Env             string
	MetaAccessToken string
	GeminiAPIKey    string
}

// LoadConfig carrega as configurações do arquivo .env e das variáveis de ambiente do sistema
func LoadConfig() *Config {
	loadDotEnv(".env")

	cfg := &Config{
		Port:            getEnv("PORT", "8080"),
		Env:             getEnv("ENV", "development"),
		MetaAccessToken: getEnv("META_ACCESS_TOKEN", ""),
		GeminiAPIKey:    getEnv("GEMINI_API_KEY", ""),
	}

	return cfg
}

// CleanToken sanitiza tokens de API removendo espaços, quebras de linha, aspas e prefixos comuns
func CleanToken(val string) string {
	val = strings.TrimSpace(val)
	val = strings.Trim(val, `"'`+"`")
	val = strings.TrimSpace(val)
	if strings.HasPrefix(strings.ToLower(val), "bearer ") {
		val = strings.TrimSpace(val[7:])
	}
	return val
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return CleanToken(val)
	}
	return defaultVal
}

func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := CleanToken(parts[1])
			_ = os.Setenv(key, val)
		}
	}
}
