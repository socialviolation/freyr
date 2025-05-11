package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/rs/zerolog/log"
	"github.com/socialviolation/freyr/shared/middlewares"
	"github.com/socialviolation/freyr/shared/telemetry"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

const service = "freyr/conscript"

var (
	tracer = otel.GetTracerProvider().Tracer("conscript")
	//meter  = otel.GetMeterProvider().Meter("conscript")
)

func conscriptRequest(ctx context.Context, url string) error {
	ctx, span := tracer.Start(ctx, "conscript_enlist_request")
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/enlist", url), nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		span.AddEvent("enlist_failed")
		return err
	}
	log.Info().Msgf("enlisted to %s - %d", url, res.StatusCode)

	span.AddEvent("enlist_success", trace.WithAttributes(attribute.String("captain", url)))
	span.End()
	return nil
}

func scheduleConscription(url string, d time.Duration) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			ctx := context.Background()
			ctx, span := tracer.Start(ctx, "conscript_enlist")
			err := conscriptRequest(ctx, url)
			if err != nil {
				log.Error().Err(err).Msgf("error enlisting to %s", url)
			}
			span.End()
			select {
			case <-time.After(d):
			case <-stop:
				return
			}
		}
	}()

	return stop
}

func setupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middlewares.DefaultStructuredLogger())

	// get global Monitor object
	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	r.Use(otelgin.Middleware(service))

	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "okay"})
	})

	return r
}

func main() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.SetEnvPrefix("")
	viper.SetDefault("host.name", "0.0.0.0")
	viper.SetDefault("host.port", 5003)
	viper.SetDefault("captain.url", "http://freyr-captain:5001")

	url := viper.GetString("captain.url")
	ctx := context.Background()
	otelShutdown, err := telemetry.NewSDK(ctx, service)
	if err != nil {
		log.Error().Err(err).Msg("error initializing otel")
		os.Exit(1)
	}

	stopConscription := scheduleConscription(url, time.Second*1)
	defer close(stopConscription)

	r := setupRoutes()
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
		log.Info().Msgf("serving @ %s", srv.Addr)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	err = otelShutdown(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error while shutting down otel")
		os.Exit(1)
	}

	stopConscription <- true

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
