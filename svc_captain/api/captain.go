package api

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/socialviolation/freyr/shared"
	"github.com/socialviolation/freyr/shared/trig"
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
	metric     *captainMetrics
}

//go:embed docket.html.tmpl
var docketTemplate string
var (
	tracer = otel.GetTracerProvider().Tracer("captain_api")
	meter  = otel.GetMeterProvider().Meter("captain_api")
)

func NewCaptainController() (*CaptainController, error) {
	spe := os.Getenv("OPERATOR_CONFIG")
	spec := shared.OperatorSpec{}
	err := json.Unmarshal([]byte(spe), &spec)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling operator config")
	}
	log.Info().Msgf("operator spec: %+v", spec)
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("15:04:05")
		},
	}

	cc := &CaptainController{
		cycleStaleDuration: time.Second * 3,
		conscripts:         make(map[string]Conscript),
		opSpec:             spec,
		docketTmpl:         template.Must(template.New("docket").Funcs(funcMap).Parse(docketTemplate)),
	}

	cc.metric, _ = newCaptainMetrics(func(ctx context.Context, observer metric.Int64Observer) error {
		args := trig.Args{
			Min:      cc.opSpec.Trig.Min,
			Max:      cc.opSpec.Trig.Max,
			Duration: cc.opSpec.Trig.Duration,
		}
		target, _ := trig.GetValue(args)
		observer.Observe(int64(target))
		log.Info().Msgf("target observable %d", int(target))
		return nil
	}, func(ctx context.Context, observer metric.Int64Observer) error {
		observer.Observe(int64(len(cc.conscripts)))
		return nil
	})

	return cc, nil
}

func (c *CaptainController) Serve(ctx context.Context, r *gin.Engine, middlewares ...gin.HandlerFunc) {
	r.Use(middlewares...)

	r.GET("/", c.docketHtml)
	r.GET("/enlist", c.enlist)
	r.GET("/conscripts", c.docket)

	c.routinePurger(ctx)
}

func (c *CaptainController) enlist(g *gin.Context) {
	ctx := g.Request.Context()
	reqSpan := trace.SpanFromContext(otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(g.Request.Header)))
	defer reqSpan.End()
	log.Info().Msgf("enlisting %s", g.Request.RemoteAddr)

	ctx, span := tracer.Start(ctx, "enlist_handler")
	defer span.End()

	ipAttr := attribute.String("enlist.conscript_ip", g.Request.RemoteAddr)
	span.SetAttributes(ipAttr)
	_, found := c.conscripts[g.Request.RemoteAddr]
	isNewAttr := attribute.Bool("enlist.is_new", !found)
	span.SetAttributes(isNewAttr)

	if !found {
		c.metric.IncUnique(ctx)
	}

	c.conscripts[g.Request.RemoteAddr] = Conscript{
		IP:       g.Request.RemoteAddr,
		LastSeen: time.Now(),
	}
}

type docketResponse struct {
	Spec       shared.OperatorSpec  `json:"operator,omitempty"`
	Name       string               `json:"name,omitempty"`
	Namespace  string               `json:"namespace,omitempty"`
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
		Name:       os.Getenv("NAME"),
		Namespace:  os.Getenv("NAME"),
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
		Name:       os.Getenv("NAME"),
		Namespace:  os.Getenv("NAMESPACE"),
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

		if time.Since(v.LastSeen) > c.cycleStaleDuration {
			log.Debug().Msgf("Cycling stale conscripts %s", k)
			conSpan.AddEvent("conscript_remove")
			log.Debug().Msgf("purging %s", k)
			delete(c.conscripts, k)
		}
		conSpan.End()
	}
}
