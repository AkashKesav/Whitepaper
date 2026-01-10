package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/nats-io/nats.go"
	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventAccess       AuditEventType = "ACCESS"
	AuditEventPolicyCheck  AuditEventType = "POLICY_CHECK"
	AuditEventPolicyChange AuditEventType = "POLICY_CHANGE"
	AuditEventLogin        AuditEventType = "LOGIN"
	AuditEventLogout       AuditEventType = "LOGOUT"
	AuditEventAdmin        AuditEventType = "ADMIN"
	AuditEventError        AuditEventType = "ERROR"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	EventType  AuditEventType    `json:"event_type"`
	UserID     string            `json:"user_id"`
	Namespace  string            `json:"namespace"`
	Action     string            `json:"action"`
	Resource   string            `json:"resource,omitempty"`
	ResourceID string            `json:"resource_id,omitempty"`
	Effect     Effect            `json:"effect"` // ALLOW or DENY
	Reason     string            `json:"reason,omitempty"`
	IPAddress  string            `json:"ip_address,omitempty"`
	UserAgent  string            `json:"user_agent,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// AuditLogger handles audit logging to multiple backends
type AuditLogger struct {
	graphClient *graph.Client
	natsConn    *nats.Conn
	logger      *zap.Logger
	enabled     bool
	asyncMode   bool
	eventChan   chan AuditEvent
}

// AuditConfig configures the audit logger
type AuditConfig struct {
	Enabled       bool
	AsyncMode     bool
	BufferSize    int
	NATSSubject   string
	RetentionDays int
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(graphClient *graph.Client, natsConn *nats.Conn, logger *zap.Logger, config AuditConfig) *AuditLogger {
	al := &AuditLogger{
		graphClient: graphClient,
		natsConn:    natsConn,
		logger:      logger,
		enabled:     config.Enabled,
		asyncMode:   config.AsyncMode,
	}

	if config.AsyncMode {
		bufSize := config.BufferSize
		if bufSize == 0 {
			bufSize = 1000
		}
		al.eventChan = make(chan AuditEvent, bufSize)
		go al.processEvents()
	}

	return al
}

// Log logs an audit event
func (al *AuditLogger) Log(ctx context.Context, event AuditEvent) error {
	if !al.enabled {
		return nil
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate ID if not provided
	if event.ID == "" {
		event.ID = fmt.Sprintf("audit_%d", time.Now().UnixNano())
	}

	if al.asyncMode {
		select {
		case al.eventChan <- event:
			return nil
		default:
			al.logger.Warn("Audit buffer full, logging synchronously")
			return al.persistEvent(ctx, event)
		}
	}

	return al.persistEvent(ctx, event)
}

// LogAccess logs a resource access event
func (al *AuditLogger) LogAccess(ctx context.Context, userID, namespace, action, resource string, effect Effect, reason string) {
	al.Log(ctx, AuditEvent{
		EventType: AuditEventAccess,
		UserID:    userID,
		Namespace: namespace,
		Action:    action,
		Resource:  resource,
		Effect:    effect,
		Reason:    reason,
	})
}

// LogPolicyCheck logs a policy evaluation event
func (al *AuditLogger) LogPolicyCheck(ctx context.Context, userID, namespace string, action Action, resourceID string, effect Effect, reason string) {
	al.Log(ctx, AuditEvent{
		EventType:  AuditEventPolicyCheck,
		UserID:     userID,
		Namespace:  namespace,
		Action:     string(action),
		ResourceID: resourceID,
		Effect:     effect,
		Reason:     reason,
	})
}

// LogPolicyChange logs a policy modification event
func (al *AuditLogger) LogPolicyChange(ctx context.Context, userID, namespace, action, policyID string) {
	al.Log(ctx, AuditEvent{
		EventType:  AuditEventPolicyChange,
		UserID:     userID,
		Namespace:  namespace,
		Action:     action,
		ResourceID: policyID,
		Effect:     EffectAllow, // Policy changes are always logged as allowed (they happened)
	})
}

// LogLogin logs a user login event
func (al *AuditLogger) LogLogin(ctx context.Context, userID, ipAddress, userAgent string, success bool) {
	effect := EffectAllow
	reason := "Login successful"
	if !success {
		effect = EffectDeny
		reason = "Login failed"
	}
	al.Log(ctx, AuditEvent{
		EventType: AuditEventLogin,
		UserID:    userID,
		Action:    "LOGIN",
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Effect:    effect,
		Reason:    reason,
	})
}

// LogError logs an error event
func (al *AuditLogger) LogError(ctx context.Context, userID, namespace, action, errorMsg string) {
	al.Log(ctx, AuditEvent{
		EventType: AuditEventError,
		UserID:    userID,
		Namespace: namespace,
		Action:    action,
		Effect:    EffectDeny,
		Reason:    errorMsg,
	})
}

// processEvents processes audit events asynchronously
func (al *AuditLogger) processEvents() {
	for event := range al.eventChan {
		if err := al.persistEvent(context.Background(), event); err != nil {
			al.logger.Error("Failed to persist audit event", zap.Error(err), zap.String("event_id", event.ID))
		}
	}
}

// persistEvent saves an audit event to DGraph and optionally publishes to NATS
func (al *AuditLogger) persistEvent(ctx context.Context, event AuditEvent) error {
	// Persist to DGraph
	if err := al.saveToDGraph(ctx, event); err != nil {
		al.logger.Error("Failed to save audit to DGraph", zap.Error(err))
	}

	// Publish to NATS for real-time streaming
	if al.natsConn != nil {
		if err := al.publishToNATS(event); err != nil {
			al.logger.Warn("Failed to publish audit to NATS", zap.Error(err))
		}
	}

	// Also log to structured logger for immediate visibility
	al.logger.Info("AUDIT",
		zap.String("event_id", event.ID),
		zap.String("type", string(event.EventType)),
		zap.String("user", event.UserID),
		zap.String("action", event.Action),
		zap.String("effect", string(event.Effect)),
		zap.String("reason", event.Reason))

	return nil
}

// saveToDGraph persists the audit event to DGraph
func (al *AuditLogger) saveToDGraph(ctx context.Context, event AuditEvent) error {
	if al.graphClient == nil {
		return nil
	}

	metadataJSON := "{}"
	if len(event.Metadata) > 0 {
		data, _ := json.Marshal(event.Metadata)
		metadataJSON = string(data)
	}

	blankNode := "_:audit"
	nquads := fmt.Sprintf(`
		%s <dgraph.type> "AuditEvent" .
		%s <audit_id> %q .
		%s <event_type> %q .
		%s <user_id> %q .
		%s <namespace> %q .
		%s <action> %q .
		%s <resource> %q .
		%s <resource_id> %q .
		%s <effect> %q .
		%s <reason> %q .
		%s <ip_address> %q .
		%s <user_agent> %q .
		%s <metadata> %q .
		%s <created_at> "%s"^^<xs:dateTime> .
	`, blankNode, blankNode, event.ID,
		blankNode, string(event.EventType),
		blankNode, event.UserID,
		blankNode, event.Namespace,
		blankNode, event.Action,
		blankNode, event.Resource,
		blankNode, event.ResourceID,
		blankNode, string(event.Effect),
		blankNode, event.Reason,
		blankNode, event.IPAddress,
		blankNode, event.UserAgent,
		blankNode, metadataJSON,
		blankNode, event.Timestamp.Format(time.RFC3339))

	txn := al.graphClient.GetDgraphClient().NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	_, err := txn.Mutate(ctx, mu)
	return err
}

// publishToNATS publishes audit event to NATS for real-time streaming
func (al *AuditLogger) publishToNATS(event AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("audit.%s.%s", event.Namespace, string(event.EventType))
	return al.natsConn.Publish(subject, data)
}

// QueryAuditLogs retrieves audit logs with filters
func (al *AuditLogger) QueryAuditLogs(ctx context.Context, userID, namespace string, eventType AuditEventType, limit int) ([]AuditEvent, error) {
	query := `query AuditLogs($limit: int) {
		logs(func: type(AuditEvent), first: $limit, orderdesc: created_at) {
			audit_id
			event_type
			user_id
			namespace
			action
			resource
			resource_id
			effect
			reason
			ip_address
			created_at
		}
	}`

	resp, err := al.graphClient.Query(ctx, query, map[string]string{"$limit": fmt.Sprintf("%d", limit)})
	if err != nil {
		return nil, err
	}

	var result struct {
		Logs []struct {
			AuditID    string `json:"audit_id"`
			EventType  string `json:"event_type"`
			UserID     string `json:"user_id"`
			Namespace  string `json:"namespace"`
			Action     string `json:"action"`
			Resource   string `json:"resource"`
			ResourceID string `json:"resource_id"`
			Effect     string `json:"effect"`
			Reason     string `json:"reason"`
			IPAddress  string `json:"ip_address"`
			CreatedAt  string `json:"created_at"`
		} `json:"logs"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	events := make([]AuditEvent, 0, len(result.Logs))
	for _, l := range result.Logs {
		// Apply filters
		if userID != "" && l.UserID != userID {
			continue
		}
		if namespace != "" && l.Namespace != namespace {
			continue
		}
		if eventType != "" && l.EventType != string(eventType) {
			continue
		}

		timestamp, _ := time.Parse(time.RFC3339, l.CreatedAt)
		events = append(events, AuditEvent{
			ID:         l.AuditID,
			Timestamp:  timestamp,
			EventType:  AuditEventType(l.EventType),
			UserID:     l.UserID,
			Namespace:  l.Namespace,
			Action:     l.Action,
			Resource:   l.Resource,
			ResourceID: l.ResourceID,
			Effect:     Effect(l.Effect),
			Reason:     l.Reason,
			IPAddress:  l.IPAddress,
		})
	}

	return events, nil
}

// Close gracefully shuts down the audit logger
func (al *AuditLogger) Close() {
	if al.eventChan != nil {
		close(al.eventChan)
	}
}
