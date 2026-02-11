package scaleset

import "github.com/prometheus/client_golang/prometheus"

var (
	// metricScaleSetListenerRunning is a gauge for scale set listeners
	metricScaleSetListenerRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myshoes_scaleset_listener_running",
			Help: "Number of running scale set listeners per target",
		},
		[]string{"target_scope"},
	)

	// metricScaleSetDesiredRunners is a gauge for desired runners
	metricScaleSetDesiredRunners = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myshoes_scaleset_desired_runners",
			Help: "Number of desired runners per target",
		},
		[]string{"target_scope"},
	)

	// metricScaleSetActiveRunners is a gauge for active runners
	metricScaleSetActiveRunners = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myshoes_scaleset_active_runners",
			Help: "Number of active runners per target",
		},
		[]string{"target_scope"},
	)

	// metricScaleSetJobsCompletedTotal is a counter for completed jobs
	metricScaleSetJobsCompletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "myshoes_scaleset_jobs_completed_total",
			Help: "Total number of completed jobs per target",
		},
		[]string{"target_scope"},
	)

	// metricScaleSetProvisionErrorsTotal is a counter for provision errors
	metricScaleSetProvisionErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "myshoes_scaleset_provision_errors_total",
			Help: "Total number of provision errors per target",
		},
		[]string{"target_scope"},
	)
)

func init() {
	prometheus.MustRegister(
		metricScaleSetListenerRunning,
		metricScaleSetDesiredRunners,
		metricScaleSetActiveRunners,
		metricScaleSetJobsCompletedTotal,
		metricScaleSetProvisionErrorsTotal,
	)
}
