package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/socialviolation/freyr/shared/initotel"
	"github.com/socialviolation/freyr/shared/middlewares"
	"github.com/socialviolation/freyr/svc_captain/api"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const service = "freyr/captain"

func setupRoutes(ctx context.Context) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middlewares.DefaultStructuredLogger())
	r.Use(otelgin.Middleware(service))

	// get global Monitor object
	m := ginmetrics.GetMonitor()
	// +optional set metric path, default /debug/metrics
	m.SetMetricPath("/metrics")
	// +optional set slow time, default 5s
	m.SetSlowTime(5)
	// +optional set request duration, default {0.1, 0.3, 1.2, 5, 10}
	// used to p95, p99
	m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})
	// set middleware for gin
	m.Use(r)

	captainSvc, err := api.NewCaptainController()
	if err != nil {
		log.Error().Err(err).Msg("error creating captain controller")
		os.Exit(1)
	}
	captainSvc.Serve(ctx, r)

	return r
}

func main() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.SetEnvPrefix("")
	viper.SetDefault("host.port", 5001)
	viper.SetDefault("host.name", "0.0.0.0")

	otelShutdown, err := initotel.NewSDK(context.Background(), service)
	ctx := context.Background()
	ctx, cancelSchedules := context.WithCancel(ctx)

	r := setupRoutes(ctx)
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", viper.GetString("host.name"), viper.GetInt32("host.port")),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("error during listen and serve")
			return
		}
		log.Info().Msgf(
			"serving @ %s", srv.Addr)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	log.Info().Msgf("serving @ %s", srv.Addr)
	<-c
	cancelSchedules()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error while shutting down server")
		os.Exit(1)
	}

	err = otelShutdown(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error while shutting down otel")
		os.Exit(1)
	}

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
