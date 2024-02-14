package api

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/socialviolation/freyr/modes"
	"net/http"
	"time"
)

type Conscript struct {
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}

type CaptainController struct {
	timeout    time.Duration
	conscripts map[string]Conscript
}

func NewCaptainController() *CaptainController {
	return &CaptainController{
		timeout:    time.Second * 3,
		conscripts: make(map[string]Conscript),
	}
}

func (c *CaptainController) Serve(r chi.Router, middlewares ...func(http.Handler) http.Handler) {
	r.Use(middlewares...)
	r.Get("/", c.docket)
	r.Get("/enlist", c.enlist)
	r.Get("/docket", c.docket)

	c.schedulePurger()
}

func (c *CaptainController) enlist(w http.ResponseWriter, r *http.Request) {
	log.Info().Msgf("enlisting %s", r.RemoteAddr)
	c.conscripts[r.RemoteAddr] = Conscript{
		IP:       r.RemoteAddr,
		LastSeen: time.Now(),
	}
}

type docketResponse struct {
	Spec       modes.OperatorSpec   `json:"operator"`
	Total      int                  `json:"total"`
	Conscripts map[string]time.Time `json:"conscripts"`
}

func (c *CaptainController) docket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dr := docketResponse{
		Total:      len(c.conscripts),
		Conscripts: make(map[string]time.Time),
	}
	for k, v := range c.conscripts {
		dr.Conscripts[k] = v.LastSeen
	}
	err := json.NewEncoder(w).Encode(dr)
	if err != nil {
		log.Error().Err(err).Msg("error encoding conscripts")
	}
}

func (c *CaptainController) schedulePurger() chan bool {
	stop := make(chan bool)

	go func() {
		for {
			for k, v := range c.conscripts {
				if time.Since(v.LastSeen) > c.timeout {
					log.Info().Msgf("purging %s", k)
					delete(c.conscripts, k)
				}
			}

			select {
			case <-time.After(c.timeout):
			case <-stop:
				return
			}
		}
	}()

	return stop
}
