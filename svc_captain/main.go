package main

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/socialviolation/freyr/svc_captain/api"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const (
	serviceName = "freyr-captain"
	defaultPort = 5001
)

func setupRoutes() *chi.Mux {

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.Heartbeat("/ping"))
	cptn := api.NewCaptainController()
	cptn.Serve(r)

	return r
}

func main() {
	port := viper.GetInt32("host.port")
	if port <= 0 {
		port = defaultPort
		log.Debug().Msgf("using default port := %d", port)
	}

	r := setupRoutes()
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", viper.GetString("host.name"), port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("error during listen and serve")
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	log.Info().Msgf("serving @ %s", srv.Addr)
	<-c
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := srv.Shutdown(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error while shutting down")
		os.Exit(1)
	}

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
