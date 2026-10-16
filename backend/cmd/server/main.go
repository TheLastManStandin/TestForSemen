// Command server runs the REST API and the Telegram bot.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbot "github.com/go-telegram/bot"
	"golang.org/x/sync/errgroup"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/bot"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/config"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/seed"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/stt"
)

const (
	shutdownTimeout = 10 * time.Second
	demoDays        = 60
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}
	store := postgres.New(pool)

	bus := events.NewBus()
	svc := expense.NewService(store, bus, cfg.Location, time.Now)
	if err := svc.SyncMembers(ctx, cfg.Members); err != nil {
		return fmt.Errorf("sync members: %w", err)
	}
	if cfg.SeedDemo {
		n, err := seed.Demo(ctx, svc, cfg.Members, demoDays, rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 42)))
		if err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
		log.Info("demo data seeded", "expenses", n)
	}

	llmClient := llm.NewOpenAI(llm.OpenAIConfig{
		BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Timeout: cfg.LLMTimeout, MaxRetries: 2,
		Referer: "https://github.com/TheLastManStandin/TestForSemen", Title: "Family Expenses Bot",
	})
	ag := agent.New(llmClient, svc, store, store, agent.Config{Model: cfg.LLMModel, Temperature: 0.1}, log)

	var transcriber stt.Transcriber // nil disables voice messages
	if cfg.STTEnabled {
		transcriber = stt.NewOpenRouter(stt.OpenRouterConfig{
			BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.STTModel, Language: "ru", Timeout: cfg.LLMTimeout,
		})
	}

	tg, err := bot.NewTelegram(cfg.TelegramToken, log, tgbot.WithServerURL(cfg.TelegramAPIURL))
	if err != nil {
		return err
	}
	tg.SetHandler(bot.NewHandler(tg, ag, bot.NewMemberSet(cfg.Members), transcriber, bot.Config{
		MaxVoiceSeconds: cfg.STTMaxSeconds, Traces: store, STTModel: cfg.STTModel,
	}, log))

	// SSE streams end through streamsCtx on shutdown. The server's request contexts must
	// not derive from the signal context, or in-flight requests would be aborted at once.
	streamsCtx, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandler(httpapi.Deps{
			Service: svc, Traces: store, Bus: bus, Health: pool.Ping, Log: log, StreamsCtx: streamsCtx,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: it would cut long-lived SSE connections.
	}
	srv.RegisterOnShutdown(endStreams)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		log.Info("telegram bot polling")
		tg.Start(gctx) // returns when gctx is cancelled
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})
	return g.Wait() // the pool is closed by the deferred Close after this returns
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
