package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/api"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	"github.com/spf13/viper"

	_ "modernc.org/sqlite"
)

func main() {
	configPath := flag.String("config", "configs/local.yaml", "config file path")
	flag.Parse()

	if err := run(*configPath); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	// Load config
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Database
	dsn := viper.GetString("database.dsn")
	if dsn == "" {
		dsn = "./data/aisre.db"
	}
	if err := os.MkdirAll("./data", 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("database ready", "dsn", dsn)

	// Repositories
	incidentRepo := store.NewIncidentRepo(db)
	reportRepo := store.NewReportRepo(db)
	evidenceRepo := store.NewEvidenceRepo(db)
	feedbackRepo := store.NewFeedbackRepo(db)

	// Services
	incidentSvc := incident.NewService(incidentRepo)

	// LLM Client (OpenAI Compatible)
	llmCfg := analysis.LLMConfig{
		BaseURL:   viper.GetString("llm.base_url"),
		APIKey:    viper.GetString("llm.api_key"),
		Model:     viper.GetString("llm.models.rca.model"),
		MaxTokens: viper.GetInt("llm.models.rca.max_tokens"),
	}
	llmClient := analysis.NewLLMClient(llmCfg)

	rcaSvc := analysis.NewRCAService(analysis.RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incidentRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Logger:       slog.Default(),
	})

	// HTTP Server
	router := api.NewRouterFull(incidentSvc, rcaSvc, feedbackRepo, reportRepo)
	addr := fmt.Sprintf("%s:%d",
		viper.GetString("server.host"),
		viper.GetInt("server.port"),
	)
	if addr == ":0" {
		addr = "0.0.0.0:8080"
	}

	slog.Info("server starting", "addr", addr)
	return http.ListenAndServe(addr, router)
}
