package main

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"

	"github.com/rameeznkhan/my-service/api/services/api/debug"
	"github.com/rameeznkhan/my-service/api/services/sales/mux"
	"github.com/rameeznkhan/my-service/foundation/logger"
)

var build = "develop"
var tag = "develop"

func main() {
	var log *logger.Logger

	events := logger.Events{
		Error: func(ctx context.Context, r logger.Record) {
			log.Info(ctx, "******* SEND ALERT ******")
		},
	}

	traceIDFn := func(ctx context.Context) string {
		return "" //web.GetTraceID(ctx)
	}

	log = logger.NewWithEvents(os.Stdout, logger.LevelInfo, "SALES", traceIDFn, events)

	ctx := context.Background()

	if err := run(ctx, log); err != nil {
		log.Error(ctx, "startup", "msg", err)
		os.Exit(1)
	}

}

func run(ctx context.Context, log *logger.Logger) error {
	log.Info(ctx, "startup", "GOMAXPROCS", runtime.GOMAXPROCS(0), "build", build)

	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout        time.Duration `conf:"default:5s"`
			WriteTimeout       time.Duration `conf:"default:10s"`
			IdleTimeout        time.Duration `conf:"default:120s"`
			ShutdownTimeout    time.Duration `conf:"default:20s"`
			APIHost            string        `conf:"default:0.0.0.0:3000"`
			DebugHost          string        `conf:"default:0.0.0.0:3010"`
			CORSAllowedOrigins []string      `conf:"default:*"`
		}
		Auth struct {
			Host      string `conf:"default:http://auth-service:6000"`
			Mechanism string `conf:"default:http"`
			GRPC      struct {
				Host string `conf:"default:auth-service:6001"`
			}
		}
		DB struct {
			User         string `conf:"default:postgres"`
			Password     string `conf:"default:postgres,mask"`
			Host         string `conf:"default:database-service"`
			Name         string `conf:"default:postgres"`
			MaxIdleConns int    `conf:"default:0"`
			MaxOpenConns int    `conf:"default:0"`
			DisableTLS   bool   `conf:"default:true"`
		}
		Tempo struct {
			Host        string  `conf:"default:tempo:4317"`
			ServiceName string  `conf:"default:sales"`
			Probability float64 `conf:"default:0.05"`
			// Shouldn't use a high Probability value in non-developer systems.
			// 0.05 should be enough for most systems. Some might want to have
			// this even lower.
		}
	}{
		Build: tag,
		Desc:  "Sales",
	}

	const prefix = "SALES"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	log.Info(ctx, "starting service", "version", cfg.Build)
	defer log.Info(ctx, "shutdown complete")
	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output : %w", err)
	}

	log.Info(ctx, "startup", "config", out)

	expvar.NewString("build").Set(cfg.Build)

	go func() {
		log.Info(ctx, "startup", "status", "debug v1 router started", "host", cfg.Web.DebugHost)

		if err := http.ListenAndServe(cfg.Web.DebugHost, debug.Mux()); err != nil {
			log.Error(ctx, "shutdown", "status", "debug v1 router closed", "host", cfg.Web.DebugHost, "msg", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	api := http.Server{
		Addr:         cfg.Web.APIHost,
		Handler:      mux.WebAPI(),
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     logger.NewStdLogger(log, logger.LevelError),
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Info(ctx, "startup", "status", "api router started", "host", api.Addr)
		serverErrors <- api.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info(ctx, "shutdown", "status", "shutdown started", "signal", sig)
		defer log.Info(ctx, "shutdown", "status", "shutdown complete", "signal", sig)

		ctx, cancel := context.WithTimeout(ctx, cfg.Web.ShutdownTimeout)
		defer cancel()

		if err := api.Shutdown(ctx); err != nil {
			api.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
