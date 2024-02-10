package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func conscriptRequest(host string) error {
	log.Info().Msg("enlisting")
	_, err := http.Get(fmt.Sprintf("http://%s/enlist", host))
	if err != nil {
		return err
	}
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

func main() {
	log.Info().Msgf("host.name: %s", viper.GetString("host.name"))
	viper.SetDefault("captain.host", "localhost:5001")
	host := viper.GetString("captain.host")

	stopConscription := scheduleConscription(host, time.Second*2)
	defer close(stopConscription)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	stopConscription <- true

	log.Info().Msg("gracefully shut down")
	os.Exit(0)
}
