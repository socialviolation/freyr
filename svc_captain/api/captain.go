package api

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/rs/zerolog/log"
	"github.com/socialviolation/freyr/shared"
	"github.com/socialviolation/freyr/shared/trig"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"html/template"
	"math"
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
	opSpec             shared.OperatorSpec

	docketTmpl *template.Template
}

//go:embed docket.html.tmpl
var docketTemplate string
var (
	tracer = otel.GetTracerProvider().Tracer("captain_api")
	meter  = otel.GetMeterProvider().Meter("captain_api")
)

const (
	MetricConscriptsTarget = "captain_conscripts_target"
	MetricConscriptsActual = "captain_conscripts_actual"
	MetricConscriptsUnique = "captain_conscripts_unique"
)

func NewCaptainController() (*CaptainController, error) {
	spe := os.Getenv("OPERATOR_CONFIG")
	spec := shared.OperatorSpec{}
	err := json.Unmarshal([]byte(spe), &spec)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling operator config")
	}
	log.Info().Msgf("operator spec: %+v", spec)

	cc := &CaptainController{
		cycleStaleDuration: time.Second * 3,
		conscripts:         make(map[string]Conscript),
		opSpec:             spec,
		docketTmpl:         template.Must(template.New("docket").Parse(docketTemplate)),
	}

	return cc, nil
}

func (c *CaptainController) Serve(ctx context.Context, r *gin.Engine, middlewares ...gin.HandlerFunc) {
	r.Use(middlewares...)

	r.GET("/", c.docketHtml)
	r.GET("/enlist", c.enlist)
	r.GET("/conscripts", c.docket)

	mtc := ginmetrics.Metric{
		Type:        ginmetrics.Gauge,
		Name:        "captain_conscripts_target",
		Description: "The target number of conscripts",
		Labels:      []string{},
	}
	_ = ginmetrics.GetMonitor().AddMetric(&mtc)
	mac := ginmetrics.Metric{
		Type:        ginmetrics.Gauge,
		Name:        "captain_conscripts_actual",
		Description: "The actual number of conscripts",
		Labels:      []string{},
	}
	_ = ginmetrics.GetMonitor().AddMetric(&mac)
	mue := ginmetrics.Metric{
		Type:        ginmetrics.Counter,
		Name:        "captain_conscripts_total",
		Description: "Total number of unique conscripts",
		Labels:      []string{},
	}
	err := ginmetrics.GetMonitor().AddMetric(&mue)
	if err != nil {
		log.Printf(err.Error())
	}
	_ = ginmetrics.GetMonitor().GetMetric(MetricConscriptsUnique).Add([]string{}, float64(0))

	c.routinePurger(ctx)
	c.routineMetrics(ctx)
}

func (c *CaptainController) startTrace(ctx context.Context, name string) (context.Context, trace.Span) {
	m0, _ := baggage.NewMemberRaw("metadata.name", "captain")
	b, _ := baggage.New(m0)
	wrappedCtx := baggage.ContextWithBaggage(ctx, b)
	return tracer.Start(wrappedCtx, name)
}

func (c *CaptainController) enlist(g *gin.Context) {
	_, span := tracer.Start(g.Request.Context(), "enlist")
	defer span.End()
	log.Info().Msgf("enlisting %s", g.Request.RemoteAddr)

	ipAttr := attribute.String("enlist.ip", g.Request.RemoteAddr)
	span.SetAttributes(ipAttr)
	_, found := c.conscripts[g.Request.RemoteAddr]
	isNewAttr := attribute.Bool("enlist.new", !found)
	span.SetAttributes(isNewAttr)

	if !found {
		_ = ginmetrics.GetMonitor().GetMetric(MetricConscriptsUnique).Inc([]string{})
	}

	c.conscripts[g.Request.RemoteAddr] = Conscript{
		IP:       g.Request.RemoteAddr,
		LastSeen: time.Now(),
	}
}

type docketResponse struct {
	Spec       shared.OperatorSpec  `json:"operator,omitempty"`
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

	buf := bytes.NewBufferString("")
	err := c.docketTmpl.Execute(buf, dr)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse{Message: fmt.Errorf("error rendering page: %w", err).Error()})
		return
	}

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Write(buf.Bytes())
}

func (c *CaptainController) routinePurger(ctx context.Context) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			traceCtx, span := tracer.Start(ctx, "conscripts_purge")
			c.purgeConscripts(traceCtx)
			span.End()

			select {
			case <-time.After(c.cycleStaleDuration):
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return stop
}

func (c *CaptainController) purgeConscripts(ctx context.Context) {
	for k, v := range c.conscripts {
		_, conSpan := tracer.Start(ctx, "conscript_check")
		conSpan.SetAttributes(attribute.String("conscript_ip", v.IP))

		log.Debug().Msgf("Cycling stale conscripts %s", k)
		if time.Since(v.LastSeen) > c.cycleStaleDuration {
			conSpan.AddEvent("conscript_remove")
			log.Debug().Msgf("purging %s", k)
			delete(c.conscripts, k)
		}
		conSpan.End()
	}
}

func (c *CaptainController) routineMetrics(ctx context.Context) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			args := trig.Args{
				Min:      c.opSpec.Trig.Min,
				Max:      c.opSpec.Trig.Max,
				Duration: c.opSpec.Trig.Duration,
			}
			target, _ := trig.GetValue(args)
			_ = ginmetrics.GetMonitor().GetMetric(MetricConscriptsTarget).SetGaugeValue([]string{}, math.Floor(target))
			_ = ginmetrics.GetMonitor().GetMetric(MetricConscriptsActual).SetGaugeValue([]string{}, float64(len(c.conscripts)))

			select {
			case <-time.After(1 * time.Second):
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return stop
}
