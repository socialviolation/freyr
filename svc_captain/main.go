package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/socialviolation/freyr/svc_captain/api"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func setupRoutes() *gin.Engine {
	r := gin.Default()
	captainSvc, err := api.NewCaptainController()
	if err != nil {
		log.Error().Err(err).Msg("error creating captain controller")
		os.Exit(1)
	}
	captainSvc.Serve(r)

	return r
}

func main() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.SetEnvPrefix("")
	viper.SetDefault("host.port", 5001)
	viper.SetDefault("host.name", "0.0.0.0")

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
