package main

import "github.com/prometheus/client_golang/prometheus"

type rcaJob struct {
	alert        Alert
	action       string
	verification string
}

// RCAQueue bounds concurrent RCA drafting so an incident storm cannot fan out into
// unbounded LLM calls, memory pressure, or downstream rate-limit failures.
type RCAQueue struct {
	jobs chan rcaJob
}

var rcaQueueTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "remediator_rca_queue_total",
		Help: "RCA work queue decisions, by outcome (enqueued/dropped).",
	},
	[]string{"outcome"},
)

func init() { prometheus.MustRegister(rcaQueueTotal) }

func NewRCAQueue(workers, depth int) *RCAQueue {
	if workers < 1 {
		workers = 1
	}
	if depth < 1 {
		depth = 1
	}
	return &RCAQueue{jobs: make(chan rcaJob, depth)}
}

func (q *RCAQueue) Start(workers int) {
	if q == nil {
		return
	}
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go func() {
			for job := range q.jobs {
				draftRCA(job.alert, job.action, job.verification)
			}
		}()
	}
}

func (q *RCAQueue) Enqueue(alert Alert, action string) bool {
	return q.EnqueueWithVerification(alert, action, "")
}

func (q *RCAQueue) EnqueueWithVerification(alert Alert, action, verification string) bool {
	if q == nil {
		return false
	}
	job := rcaJob{alert: alert, action: action, verification: verification}
	select {
	case q.jobs <- job:
		rcaQueueTotal.WithLabelValues("enqueued").Inc()
		return true
	default:
		rcaQueueTotal.WithLabelValues("dropped").Inc()
		if logger != nil {
			logger.Warnw("rca queue full; dropping draft",
				"alertname", alert.alertName(),
				"incident_key", alert.incidentKey(),
			)
		}
		return false
	}
}
