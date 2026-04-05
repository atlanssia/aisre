package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
	"github.com/atlanssia/aisre/internal/alertgroup"
	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/api"
	"github.com/atlanssia/aisre/internal/change"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/postmortem"
	"github.com/atlanssia/aisre/internal/promptstudio"
	"github.com/atlanssia/aisre/internal/similar"
	"github.com/atlanssia/aisre/internal/store"
	"github.com/atlanssia/aisre/internal/tool"
	"github.com/atlanssia/aisre/internal/topology"
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
	_ = viper.BindEnv("embedding.base_url", "EMBEDDING_BASE_URL")
	_ = viper.BindEnv("embedding.api_key", "EMBEDDING_API_KEY")
	_ = viper.BindEnv("embedding.model", "EMBEDDING_MODEL")
	_ = viper.BindEnv("embedding.dimensions", "EMBEDDING_DIMENSIONS")
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
	defer func() { _ = db.Close() }()

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

	// Topology / Blast Radius Service (Phase 2, feature-flagged)
	var topoSvc *topology.Service
	if viper.GetBool("features.topology.enabled") {
		topoRepo := store.NewTopologyRepo(db)
		topoSvc = topology.NewService(topoRepo, incidentRepo)
		slog.Info("topology feature enabled")
	}

	// Prompt Studio Service (Phase 2, feature-flagged)
	var promptStudioSvc *promptstudio.Service
	if viper.GetBool("features.prompt_studio.enabled") {
		promptTplRepo := store.NewPromptTemplateRepo(db)
		promptStudioSvc = promptstudio.NewService(promptTplRepo)
		slog.Info("prompt studio feature enabled")
	}

	// Alert Aggregation Service (Phase 2, feature-flagged)
	var alertGroupSvc *alertgroup.Service
	if viper.GetBool("features.alert_aggregation.enabled") {
		alertGroupRepo := store.NewAlertGroupRepo(db)
		alertGroupSvc = alertgroup.NewService(alertGroupRepo, incidentSvc)
		slog.Info("alert aggregation feature enabled")
	}

	// Postmortem Service (Phase 2, feature-flagged)
	var postmortemSvc *postmortem.Service
	if viper.GetBool("features.postmortem.enabled") {
		pmRepo := store.NewPostmortemRepo(db)
		llmGen := postmortem.NewDefaultLLMGenerator(func(ctx context.Context, messages []postmortem.Message) (*postmortem.LLMResponse, error) {
			resp, err := llmClient.Complete(ctx, convertMessages(messages))
			if err != nil {
				return nil, err
			}
			return &postmortem.LLMResponse{Content: resp.Content}, nil
		})
		postmortemSvc = postmortem.NewService(pmRepo, incidentSvc, reportRepo, evidenceRepo, feedbackRepo, llmGen)
		slog.Info("postmortem feature enabled")
	}

	rcaSvc := analysis.NewRCAService(analysis.RCAServiceConfig{
		LLMClient:      llmClient,
		IncidentRepo:   incidentRepo,
		ReportRepo:     reportRepo,
		EvidenceRepo:   evidenceRepo,
		Orchestrator:   orchestrator,
		SimilarFinder:  similarSvc,
		ChangeFinder:   changeSvc,
		TopologyFinder: topoSvc,
		Logger:         slog.Default(),
	})

	// HTTP Server
	staticFS := getStaticFS()
	router := api.NewRouterFullWithPostmortem(incidentSvc, rcaSvc, feedbackRepo, reportRepo, similarSvc, changeSvc, topoSvc, promptStudioSvc, alertGroupSvc, postmortemSvc, staticFS)
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

// convertMessages adapts postmortem.Message slices to analysis.Message for the LLM client.
func convertMessages(msgs []postmortem.Message) []analysis.Message {
	result := make([]analysis.Message, len(msgs))
	for i, m := range msgs {
		result[i] = analysis.Message{Role: m.Role, Content: m.Content}
	}
	return result
}
