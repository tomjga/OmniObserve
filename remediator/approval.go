package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalDenied   ApprovalStatus = "denied"
)

type ApprovalRequest struct {
	ID          string
	Status      ApprovalStatus
	Alert       Alert
	Flag        string
	Action      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DecidedBy   string
	DecisionMsg string
	Outcome     Outcome
}

type ApprovalView struct {
	ID          string         `json:"id"`
	Status      ApprovalStatus `json:"status"`
	AlertName   string         `json:"alertname"`
	Service     string         `json:"service"`
	IncidentKey string         `json:"incident_key"`
	Flag        string         `json:"flag"`
	Action      string         `json:"action"`
	Summary     string         `json:"summary"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DecidedBy   string         `json:"decided_by,omitempty"`
	DecisionMsg string         `json:"decision_note,omitempty"`
	Outcome     string         `json:"outcome,omitempty"`
	CanApprove  bool           `json:"can_approve"`
}

func (r ApprovalRequest) View() ApprovalView {
	return ApprovalView{
		ID:          r.ID,
		Status:      r.Status,
		AlertName:   r.Alert.alertName(),
		Service:     r.Alert.serviceName(),
		IncidentKey: r.Alert.incidentKey(),
		Flag:        r.Flag,
		Action:      r.Action,
		Summary:     r.Alert.Annotations["summary"],
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		DecidedBy:   r.DecidedBy,
		DecisionMsg: r.DecisionMsg,
		Outcome:     string(r.Outcome),
		CanApprove:  r.Status == ApprovalPending,
	}
}

type ApprovalStore struct {
	mu    sync.Mutex
	items map[string]ApprovalRequest
	byKey map[string]string
}

func NewApprovalStore() *ApprovalStore {
	return &ApprovalStore{
		items: map[string]ApprovalRequest{},
		byKey: map[string]string{},
	}
}

func (s *ApprovalStore) Create(alert Alert, flag string) (ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := approvalKey(alert, flag)
	if id, ok := s.byKey[key]; ok {
		item := s.items[id]
		if item.Status == ApprovalPending {
			return item, false
		}
	}

	now := time.Now().UTC()
	item := ApprovalRequest{
		ID:        approvalID(alert, flag, now),
		Status:    ApprovalPending,
		Alert:     alert,
		Flag:      flag,
		Action:    "disable-flag",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.items[item.ID] = item
	s.byKey[key] = item.ID
	return item, true
}

func (s *ApprovalStore) List(status ApprovalStatus) []ApprovalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]ApprovalRequest, 0, len(s.items))
	for _, item := range s.items {
		if status != "" && item.Status != status {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

func (s *ApprovalStore) Get(id string) (ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	return item, ok
}

func (s *ApprovalStore) Approve(id, actor, note string) (ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok || item.Status != ApprovalPending {
		return item, false
	}
	item.Status = ApprovalApproved
	item.DecidedBy = actor
	item.DecisionMsg = note
	item.UpdatedAt = time.Now().UTC()
	s.items[id] = item
	delete(s.byKey, approvalKey(item.Alert, item.Flag))
	return item, true
}

func (s *ApprovalStore) Deny(id, actor, note string) (ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok || item.Status != ApprovalPending {
		return item, false
	}
	item.Status = ApprovalDenied
	item.DecidedBy = actor
	item.DecisionMsg = note
	item.Outcome = OutcomeNeedsHuman
	item.UpdatedAt = time.Now().UTC()
	s.items[id] = item
	delete(s.byKey, approvalKey(item.Alert, item.Flag))
	return item, true
}

func (s *ApprovalStore) SetOutcome(id string, outcome Outcome) (ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return item, false
	}
	item.Outcome = outcome
	item.UpdatedAt = time.Now().UTC()
	s.items[id] = item
	return item, true
}

func (s *ApprovalStore) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, item := range s.items {
		if item.Status == ApprovalPending {
			count++
		}
	}
	return count
}

func approvalKey(alert Alert, flag string) string {
	return alert.incidentKey() + "|" + flag
}

func approvalID(alert Alert, flag string, at time.Time) string {
	sum := sha1.Sum([]byte(approvalKey(alert, flag) + "|" + at.Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])[:16]
}

type approvalDecisionBody struct {
	Actor string `json:"actor"`
	Note  string `json:"note"`
}

func parseApprovalStatus(raw string) (ApprovalStatus, bool) {
	switch ApprovalStatus(raw) {
	case "", ApprovalPending, ApprovalApproved, ApprovalDenied:
		return ApprovalStatus(raw), true
	default:
		return "", false
	}
}

var approvalStore *ApprovalStore

var approvalDecisionsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "remediator_approvals_total",
		Help: "Human approval gate decisions, by flag and decision.",
	},
	[]string{"flag", "decision"},
)

var pendingApprovalsGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "remediator_pending_approvals",
		Help: "Approval requests waiting for a human decision.",
	},
)

func init() {
	prometheus.MustRegister(approvalDecisionsTotal, pendingApprovalsGauge)
}

func ensureApprovalStore() *ApprovalStore {
	if approvalStore == nil {
		approvalStore = NewApprovalStore()
	}
	return approvalStore
}

func queueApproval(alert Alert, flag string) ApprovalRequest {
	store := ensureApprovalStore()
	item, created := store.Create(alert, flag)
	if created {
		approvalDecisionsTotal.WithLabelValues(flag, string(ApprovalPending)).Inc()
		refreshPendingApprovalsGauge()
	}
	return item
}

func refreshPendingApprovalsGauge() {
	if approvalStore == nil {
		pendingApprovalsGauge.Set(0)
		return
	}
	pendingApprovalsGauge.Set(float64(approvalStore.PendingCount()))
}

func listApprovalsHandler(c *gin.Context) {
	status, ok := parseApprovalStatus(c.Query("status"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid approval status"})
		return
	}
	items := ensureApprovalStore().List(status)
	views := make([]ApprovalView, 0, len(items))
	for _, item := range items {
		views = append(views, item.View())
	}
	c.JSON(http.StatusOK, gin.H{"approvals": views})
}

func approveApprovalHandler(c *gin.Context) {
	var body approvalDecisionBody
	_ = c.ShouldBindJSON(&body)
	if body.Actor == "" {
		body.Actor = "human"
	}

	store := ensureApprovalStore()
	id := c.Param("id")
	item, ok := store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
		return
	}
	if item.Status != ApprovalPending {
		c.JSON(http.StatusConflict, gin.H{"error": "approval already decided", "approval": item.View()})
		return
	}
	if flagRemediator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "remediator inactive; no cluster action client is available", "approval": item.View()})
		return
	}

	item, ok = store.Approve(id, body.Actor, body.Note)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "approval already decided"})
		return
	}
	refreshPendingApprovalsGauge()
	approvalDecisionsTotal.WithLabelValues(item.Flag, string(ApprovalApproved)).Inc()

	outcome := executeApprovedRemediation(c.Request.Context(), item, trace.SpanFromContext(c.Request.Context()))
	item, _ = store.SetOutcome(id, outcome)
	if outcome == OutcomeFailed {
		c.JSON(http.StatusInternalServerError, gin.H{"approval": item.View()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"approval": item.View()})
}

func denyApprovalHandler(c *gin.Context) {
	var body approvalDecisionBody
	_ = c.ShouldBindJSON(&body)
	if body.Actor == "" {
		body.Actor = "human"
	}

	item, ok := ensureApprovalStore().Deny(c.Param("id"), body.Actor, body.Note)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pending approval not found"})
		return
	}
	refreshPendingApprovalsGauge()
	approvalDecisionsTotal.WithLabelValues(item.Flag, string(ApprovalDenied)).Inc()
	c.JSON(http.StatusOK, gin.H{"approval": item.View()})
}

func executeApprovedRemediation(ctx context.Context, item ApprovalRequest, span trace.Span) Outcome {
	var before []ObservedMetric
	if verifier != nil {
		before = verifier.Gather(ctx, item.Alert.serviceName())
	}

	outcome, err := flagRemediator.DisableFlag(ctx, item.Flag, item.Alert.incidentKey())
	if err != nil {
		outcome = OutcomeFailed
		logger.Errorw("approved remediation failed", "approval_id", item.ID, "flag", item.Flag, "incident_key", item.Alert.incidentKey(), "error", err)
	} else if outcome == OutcomeAlreadySafe && noopGuard != nil && noopGuard.Record(item.Flag, item.Alert.incidentKey()) {
		outcome = OutcomeNeedsHuman
		noopStormsTotal.WithLabelValues(item.Flag).Inc()
		logger.Warnw("approved remediation hit no-op storm",
			"approval_id", item.ID,
			"flag", item.Flag,
			"incident_key", item.Alert.incidentKey(),
			"threshold", noopGuard.threshold,
			"window", noopGuard.window.String())
	} else {
		logger.Infow("approved remediation",
			"approval_id", item.ID,
			"flag", item.Flag,
			"outcome", string(outcome),
			"incident_key", item.Alert.incidentKey())
	}
	recordAction(item.Flag, outcome, span)
	span.SetAttributes(
		attribute.String("approval.id", item.ID),
		attribute.String("approval.status", string(ApprovalApproved)),
	)
	if outcome == OutcomeHealed {
		verifyAndDraftAfterAction(item.Alert, item.Flag, before)
	}
	return outcome
}
