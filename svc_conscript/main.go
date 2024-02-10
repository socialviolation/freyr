package main

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"time"
)

func scheduleConscription(d time.Duration) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			log.Info().Msgf("Polling")
			select {
			case <-time.After(d):
			case <-stop:
				return
			}
		}
	}()

	return stop
}

func main() {
	log.Info().Msgf("host.name: %s", viper.GetString("host.name"))

	stopConscription := scheduleConscription(time.Second * 2)
	defer close(stopConscription)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	stopConscription <- true

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
