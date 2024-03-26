package api

import (
	"context"
	"go.opentelemetry.io/otel/metric"
)

const (
	MetricConscriptsTarget = "conscripts.target"
	MetricConscriptsActual = "conscripts.actual"
	MetricConscriptsUnique = "conscripts.unique"
)

type captainMetrics struct {
	conscriptsTarget metric.Int64ObservableGauge
	conscriptsActual metric.Int64ObservableGauge
	conscriptsUnique metric.Int64Counter
}

func newCaptainMetrics(targetCB metric.Int64Callback, actualCB metric.Int64Callback) (*captainMetrics, error) {
	var err error

	cm := captainMetrics{}

	cm.conscriptsTarget, err = meter.Int64ObservableGauge(MetricConscriptsTarget,
		metric.WithDescription("The target number of conscripts"),
		metric.WithUnit("{conscripts}"),
		metric.WithInt64Callback(targetCB))
	if err != nil {
		return nil, err
	}

	cm.conscriptsActual, err = meter.Int64ObservableGauge(MetricConscriptsActual,
		metric.WithDescription("The actual number of conscripts"),
		metric.WithUnit("{conscripts}"),
		metric.WithInt64Callback(actualCB))
	if err != nil {
		return nil, err
	}

	cm.conscriptsUnique, err = meter.Int64Counter(MetricConscriptsUnique,
		metric.WithDescription("The unique number of conscripts"),
		metric.WithUnit("{conscripts}"))
	if err != nil {
		return nil, err
	}

	return &cm, nil
}

func (c *captainMetrics) IncUnique(ctx context.Context) {
	c.conscriptsUnique.Add(ctx, 1)
}
