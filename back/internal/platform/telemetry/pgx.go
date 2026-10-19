package telemetry

import (
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// PgxTracer returns a pgx query tracer that creates OpenTelemetry spans.
// Query parameters are not recorded (they may contain password hashes).
func (t *Telemetry) PgxTracer() pgx.QueryTracer {
	return otelpgx.NewTracer(otelpgx.WithTracerProvider(t.tp))
}

// PoolCollector returns a Prometheus collector exposing pgxpool statistics.
// Register it with Telemetry.Register.
func PoolCollector(pool *pgxpool.Pool) prometheus.Collector {
	return &poolCollector{pool: pool}
}

// PoolCollector is a method shortcut for the package-level PoolCollector.
func (t *Telemetry) PoolCollector(pool *pgxpool.Pool) prometheus.Collector {
	return PoolCollector(pool)
}

var (
	poolAcquiredDesc = prometheus.NewDesc("pgxpool_acquired_conns", "Currently acquired connections.", nil, nil)
	poolIdleDesc     = prometheus.NewDesc("pgxpool_idle_conns", "Currently idle connections.", nil, nil)
	poolTotalDesc    = prometheus.NewDesc("pgxpool_total_conns", "Total connections in the pool.", nil, nil)
	poolMaxDesc      = prometheus.NewDesc("pgxpool_max_conns", "Maximum size of the pool.", nil, nil)
	poolAcqCountDesc = prometheus.NewDesc("pgxpool_acquire_count_total", "Cumulative count of successful acquires.", nil, nil)
	poolAcqDurDesc   = prometheus.NewDesc("pgxpool_acquire_duration_seconds_total", "Total time spent acquiring connections.", nil, nil)
	poolEmptyDesc    = prometheus.NewDesc("pgxpool_empty_acquire_count_total", "Acquires that had to wait for a connection.", nil, nil)
	poolCanceledDesc = prometheus.NewDesc("pgxpool_canceled_acquire_count_total", "Acquires canceled by context.", nil, nil)
)

type poolCollector struct {
	pool *pgxpool.Pool
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		poolAcquiredDesc, poolIdleDesc, poolTotalDesc, poolMaxDesc,
		poolAcqCountDesc, poolAcqDurDesc, poolEmptyDesc, poolCanceledDesc,
	} {
		ch <- d
	}
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(poolAcquiredDesc, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(poolIdleDesc, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(poolTotalDesc, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(poolMaxDesc, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(poolAcqCountDesc, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolAcqDurDesc, prometheus.CounterValue, s.AcquireDuration().Seconds())
	ch <- prometheus.MustNewConstMetric(poolEmptyDesc, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolCanceledDesc, prometheus.CounterValue, float64(s.CanceledAcquireCount()))
}
