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

func conscriptRequest(host string) error {
	_, err := http.Get(fmt.Sprintf("http://%s/enlist", host))
	if err != nil {
		return err
	}
	log.Info().Msg("enlisted")
	return nil
}

func scheduleConscription(host string, d time.Duration) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			err := conscriptRequest(host)
			if err != nil {
				log.Error().Err(err).Msg("error enlisting")
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
	viper.SetDefault("host.port", 5003)
	viper.SetDefault("host.name", "0.0.0.0")
	viper.SetDefault("captain.host", "localhost:5001")

	host := viper.GetString("captain.host")
	log.Info().Msgf("host.name: %s", host)

	stopConscription := scheduleConscription(host, time.Second*5)
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
		log.Info().Msg("serving on " + srv.Addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("error during listen and serve")
		}
	}()

	e := chi.Walk(r, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		fmt.Printf("[%s]: '%s' has %d middlewares\n", method, route, len(middlewares))
		return nil
	})
	if e != nil {
		log.Error().Err(e).Msg("error walking router")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	stopConscription <- true

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
