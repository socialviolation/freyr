package api

import (
	_ "embed"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/socialviolation/freyr/modes"
	"github.com/socialviolation/freyr/modes/trig"
	"html/template"
	"net/http"
	"os"
	"time"
)

type Conscript struct {
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}

type CaptainController struct {
	cycleStaleDuration time.Duration
	conscripts         map[string]Conscript
	opSpec             modes.OperatorSpec

	docketTmpl *template.Template
}

//go:embed docket.html
var docketTemplate string

func NewCaptainController() (*CaptainController, error) {
	spe := os.Getenv("OPERATOR_CONFIG")
	spec := modes.OperatorSpec{}
	err := json.Unmarshal([]byte(spe), &spec)
	if err != nil {
		log.Error().Err(err).Msg("error unmarshalling operator spec")
		return nil, err
	}
	log.Info().Msgf("operator spec: %+v", spec)

	return &CaptainController{
		cycleStaleDuration: time.Second * 3,
		conscripts:         make(map[string]Conscript),
		opSpec:             spec,
		docketTmpl:         template.Must(template.New("docket").Parse(docketTemplate)),
	}, nil
}

func (c *CaptainController) Serve(r chi.Router, middlewares ...func(http.Handler) http.Handler) {
	r.Use(middlewares...)
	r.Get("/", c.docketHtml)
	r.Get("/enlist", c.enlist)
	r.Get("/conscripts", c.docket)

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
	Conscripts map[string]time.Time `json:"conscripts"`
	Trig       string               `json:"trig,omitempty"`
	Target     int                  `json:"target,omitempty"`
	Actual     int                  `json:"actual"`
}

func (c *CaptainController) docket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	target, err := trig.GetValue(trig.Args{
		Min:      c.opSpec.Trig.Min,
		Max:      c.opSpec.Trig.Max,
		Duration: c.opSpec.Trig.Duration,
	})
	dr := docketResponse{
		Spec:       c.opSpec,
		Target:     int(target),
		Actual:     len(c.conscripts),
		Conscripts: make(map[string]time.Time),
	}

	for k, v := range c.conscripts {
		dr.Conscripts[k] = v.LastSeen
	}
	err = json.NewEncoder(w).Encode(dr)
	if err != nil {
		log.Error().Err(err).Msg("error encoding conscripts")
	}
}

func (c *CaptainController) docketHtml(w http.ResponseWriter, r *http.Request) {
	dr := docketResponse{
		Spec:       c.opSpec,
		Actual:     len(c.conscripts),
		Conscripts: make(map[string]time.Time),
	}

	for k, v := range c.conscripts {
		dr.Conscripts[k] = v.LastSeen
	}
	if c.opSpec.Mode == "trig" {
		args := trig.Args{
			Min:      c.opSpec.Trig.Min,
			Max:      c.opSpec.Trig.Max,
			Duration: c.opSpec.Trig.Duration,
		}
		target, _ := trig.GetValue(args)
		dr.Target = int(target)
		dr.Trig = trig.RenderChart(args)
	}

	err := c.docketTmpl.Execute(w, dr)
	if err != nil {
		return
	}
}

func (c *CaptainController) schedulePurger() chan bool {
	stop := make(chan bool)

	go func() {
		for {
			for k, v := range c.conscripts {
				log.Debug().Msgf("Cycling stale conscripts %s", k)
				if time.Since(v.LastSeen) > c.cycleStaleDuration {
					log.Debug().Msgf("purging %s", k)
					delete(c.conscripts, k)
				}
			}

			select {
			case <-time.After(c.cycleStaleDuration):
			case <-stop:
				return
			}
		}
	}()

	return stop
}
