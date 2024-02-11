package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func conscriptRequest(url string) error {
	_, err := http.Get(fmt.Sprintf("%s/enlist", url))
	if err != nil {
		return err
	}
	log.Info().Msgf("enlisted to %s", url)
	return nil
}

func scheduleConscription(url string, d time.Duration) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			err := conscriptRequest(url)
			if err != nil {
				log.Error().Err(err).Msgf("error enlisting to %s", url)
			}
			select {
			case <-time.After(d):
			case <-stop:
				return
			}
		}
	}()

	return stop
}

func setupRoutes() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.Heartbeat("/ping"))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("."))
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
	stopConscription := scheduleConscription(url, time.Second*5)
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
	stopConscription <- true

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
