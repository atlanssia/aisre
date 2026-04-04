package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/api"
	"github.com/atlanssia/aisre/internal/change"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/similar"
	"github.com/atlanssia/aisre/internal/store"
	"github.com/atlanssia/aisre/internal/tool"
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
	// Bind env vars for embedding config (AutomaticEnv alone doesn't map underscores to dots)
	viper.BindEnv("embedding.base_url", "EMBEDDING_BASE_URL")
	viper.BindEnv("embedding.api_key", "EMBEDDING_API_KEY")
	viper.BindEnv("embedding.model", "EMBEDDING_MODEL")
	viper.BindEnv("embedding.dimensions", "EMBEDDING_DIMENSIONS")
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

	// OO Adapter
	ooCfg := openobserve.Config{
		BaseURL:  viper.GetString("adapters.openobserve.base_url"),
		OrgID:    viper.GetString("adapters.openobserve.org_id"),
		Token:    viper.GetString("adapters.openobserve.token"),
		Username: viper.GetString("adapters.openobserve.username"),
		Password: viper.GetString("adapters.openobserve.password"),
	}

	// Tool Orchestrator — wire OO adapter into tools
	var orchestrator *tool.Orchestrator
	ooClient, err := openobserve.NewClient(ooCfg, slog.Default())
	if err != nil {
		slog.Warn("OO adapter not available, running without tool evidence", "error", err)
	} else {
		stream := viper.GetString("adapters.openobserve.stream")
		if stream == "" {
			stream = "default"
		}
		tools := []tool.Tool{
			tool.NewLogsTool(tool.LogsToolConfig{Provider: ooClient, Stream: stream}),
			tool.NewTraceTool(tool.TraceToolConfig{Provider: ooClient, Stream: stream}),
			tool.NewMetricsTool(tool.MetricsToolConfig{Provider: ooClient, Stream: stream}),
		}
		orchestrator = tool.NewOrchestrator(tools, slog.Default())
		slog.Info("OO adapter wired", "base_url", ooCfg.BaseURL, "stream", stream)
	}

	// Similar Incident Service (Phase 2, feature-flagged)
	var similarSvc *similar.Service
	if viper.GetBool("features.similar_incident.enabled") {
		embCfg := analysis.EmbeddingConfig{
			BaseURL:    viper.GetString("embedding.base_url"),
			APIKey:     viper.GetString("embedding.api_key"),
			Model:      viper.GetString("embedding.model"),
			Dimensions: viper.GetInt("embedding.dimensions"),
		}
		if embCfg.BaseURL == "" {
			// Fallback: reuse main LLM config as embedding provider
			embCfg.BaseURL = llmCfg.BaseURL
			embCfg.APIKey = llmCfg.APIKey
			embCfg.Model = "text-embedding-3-small"
		}
		embClient := analysis.NewEmbeddingClient(embCfg)
		embRepo := store.NewEmbeddingRepo(db)
		similarSvc = similar.NewService(embClient, embRepo, incidentRepo, reportRepo)
		slog.Info("similar incident feature enabled", "model", embCfg.Model)
	}

	// Change Correlation Service (Phase 2, feature-flagged)
	var changeSvc *change.Service
	if viper.GetBool("features.change_correlation.enabled") {
		changeRepo := store.NewChangeRepo(db)
		changeSvc = change.NewService(changeRepo, incidentRepo, reportRepo)
		slog.Info("change correlation feature enabled")
	}

	rcaSvc := analysis.NewRCAService(analysis.RCAServiceConfig{
		LLMClient:     llmClient,
		IncidentRepo:  incidentRepo,
		ReportRepo:    reportRepo,
		EvidenceRepo:  evidenceRepo,
		Orchestrator:  orchestrator,
		SimilarFinder: similarSvc,
		Logger:        slog.Default(),
	})

	// HTTP Server
	staticFS := getStaticFS()
	router := api.NewRouterFullWithChanges(incidentSvc, rcaSvc, feedbackRepo, reportRepo, similarSvc, changeSvc, staticFS)
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
