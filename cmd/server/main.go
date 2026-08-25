package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raphaelbalbinot/adlead-finder/internal/ai"
	"github.com/raphaelbalbinot/adlead-finder/internal/config"
	"github.com/raphaelbalbinot/adlead-finder/internal/db"
	"github.com/raphaelbalbinot/adlead-finder/internal/handlers"
	"github.com/raphaelbalbinot/adlead-finder/internal/meta"
)

func main() {
	fmt.Println("======================================================")
	fmt.Println("    🚀 NEXUS ADLEAD FINDER (GOLANG EDITION)          ")
	fmt.Println("======================================================")

	// 1. Carregar Configurações
	cfg := config.LoadConfig()
	log.Printf("[Config] Porta: %s | Ambiente: %s", cfg.Port, cfg.Env)

	if cfg.MetaAccessToken == "" {
		log.Println("⚠️  AVISO: META_ACCESS_TOKEN não configurado no .env")
	} else {
		log.Println("✓ META_ACCESS_TOKEN carregado com sucesso")
	}

	if cfg.GeminiAPIKey == "" {
		log.Println("⚠️  AVISO: GEMINI_API_KEY não configurada no .env")
	} else {
		log.Println("✓ GEMINI_API_KEY carregada com sucesso")
	}

	// 2. Inicializar Banco de Dados SQLite
	database, err := db.InitDB("./leads.db")
	if err != nil {
		log.Fatalf("❌ Erro fatal ao inicializar banco de dados SQLite: %v", err)
	}
	defer database.Close()

	// 3. Inicializar Clientes de API
	metaClient := meta.NewClient(cfg.MetaAccessToken)
	aiClient := ai.NewClient(cfg.GeminiAPIKey)

	// 4. Inicializar Servidor e Handlers
	server := handlers.NewServer(database, metaClient, aiClient)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.GetRouter(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 5. Iniciar Servidor em Goroutine
	go func() {
		log.Printf("🌐 Servidor rodando em: http://localhost:%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Erro ao iniciar servidor HTTP: %v", err)
		}
	}()

	// 6. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("\n🛑 Encerrando AdLead Finder de forma graciosa...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Aviso no shutdown do servidor: %v", err)
	}

	log.Println("✓ Servidor finalizado com segurança.")
}
