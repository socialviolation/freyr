package api

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/socialviolation/freyr/modes"
	"github.com/socialviolation/freyr/modes/trig"
	"net/http"
	"os"
	"time"
)

type Conscript struct {
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}

type CaptainController struct {
	timeout    time.Duration
	conscripts map[string]Conscript
	opSpec     modes.OperatorSpec
}

func NewCaptainController() (*CaptainController, error) {
	spe := os.Getenv("OPERATOR_CONFIG")
	spec := modes.OperatorSpec{}
	err := json.Unmarshal([]byte(spe), &spec)
	if err != nil {
		log.Error().Err(err).Msg("error unmarshaling operator spec")
		return nil, err
	}

	return &CaptainController{
		timeout:    time.Second * 3,
		conscripts: make(map[string]Conscript),
		opSpec:     spec,
	}, nil
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
	Spec       modes.OperatorSpec   `json:"operator,omitempty"`
	Total      int                  `json:"total"`
	Conscripts map[string]time.Time `json:"conscripts"`
	Trig       string               `json:"trig,omitempty"`
}

func (c *CaptainController) docket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dr := docketResponse{
		Spec:       c.opSpec,
		Total:      len(c.conscripts),
		Conscripts: make(map[string]time.Time),
	}
	if c.opSpec.Mode == "trig" {
		dr.Trig = trig.RenderValues(trig.Args{Duration: c.opSpec.Trig.Duration, Max: c.opSpec.Trig.Max, Min: c.opSpec.Trig.Min})
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
