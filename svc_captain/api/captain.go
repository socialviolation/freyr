package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
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

const (
	ContentTypeHTML = "text/html; charset=utf-8"
)

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
		return nil, fmt.Errorf("error unmarshalling operator config")
	}
	log.Info().Msgf("operator spec: %+v", spec)

	return &CaptainController{
		cycleStaleDuration: time.Second * 3,
		conscripts:         make(map[string]Conscript),
		opSpec:             spec,
		docketTmpl:         template.Must(template.New("docket").Parse(docketTemplate)),
	}, nil
}

func (c *CaptainController) Serve(r *gin.Engine, middlewares ...gin.HandlerFunc) {
	r.Use(middlewares...)

	r.GET("/", c.docketHtml)
	r.GET("/enlist", c.enlist)
	r.GET("/conscripts", c.docket)

	c.schedulePurger()
}

func (c *CaptainController) enlist(ctx *gin.Context) {
	log.Info().Msgf("enlisting %s", ctx.Request.RemoteAddr)
	c.conscripts[ctx.Request.RemoteAddr] = Conscript{
		IP:       ctx.Request.RemoteAddr,
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

type errorResponse struct {
	Message string
}

func (c *CaptainController) docket(ctx *gin.Context) {
	target, err := trig.GetValue(trig.Args{
		Min:      c.opSpec.Trig.Min,
		Max:      c.opSpec.Trig.Max,
		Duration: c.opSpec.Trig.Duration,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse{Message: "error calculating the target conscripts"})
	}

	dr := docketResponse{
		Spec:       c.opSpec,
		Target:     int(target),
		Actual:     len(c.conscripts),
		Conscripts: make(map[string]time.Time),
	}

	for k, v := range c.conscripts {
		dr.Conscripts[k] = v.LastSeen
	}
	ctx.JSON(http.StatusOK, dr)
}

func (c *CaptainController) docketHtml(ctx *gin.Context) {
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

	ctx.String(http.StatusOK, ContentTypeHTML, dr)
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
