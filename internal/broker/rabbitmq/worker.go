package rabbitmq

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"game-wallet-ledger/internal/domain"
	"game-wallet-ledger/internal/repository/postgres"
	"game-wallet-ledger/pkg/logger"
	"game-wallet-ledger/pkg/metrics"
)

type Worker struct {
	broker    *Broker
	eventRepo *postgres.EventRepository
}

func NewWorker(broker *Broker, eventRepo *postgres.EventRepository) *Worker {
	return &Worker{
		broker:    broker,
		eventRepo: eventRepo,
	}
}

// Start launches the consumer loop. It stops gracefully when ctx is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	ch := w.broker.Channel()

	// High throughput QoS prefetch
	if err := ch.Qos(20, 0, false); err != nil {
		return err
	}

	msgs, err := ch.ConsumeWithContext(
		ctx,
		w.broker.cfg.QueueName, // queue
		"wallet-item-fulfiller", // consumer tag
		false,                   // auto-ack (manual ack for at-least-once reliability)
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)
	if err != nil {
		return err
	}

	logger.FromContext(ctx).Info("started rabbitmq background event consumer worker", "queue", w.broker.cfg.QueueName)

	for {
		select {
		case <-ctx.Done():
			logger.FromContext(ctx).Info("shutting down background event worker")
			return nil
		case d, ok := <-msgs:
			if !ok {
				logger.FromContext(ctx).Warn("rabbitmq delivery channel closed")
				return nil
			}
			w.handleDelivery(ctx, d)
		}
	}
}

func (w *Worker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	var event domain.PaymentEvent
	if err := json.Unmarshal(d.Body, &event); err != nil {
		logger.FromContext(ctx).Error("failed to unmarshal payment event",
			"error", err.Error(),
			"body", string(d.Body),
		)
		_ = d.Reject(false) // Drop malformed poison messages
		metrics.EventsProcessedTotal.WithLabelValues("unknown", "malformed").Inc()
		return
	}

	// Idempotent event check using event_id
	newlyRecorded, err := w.eventRepo.RecordProcessed(ctx, event.EventID, event.EventType, event.TransactionID)
	if err != nil {
		logger.FromContext(ctx).Error("failed to verify event idempotency in database",
			"event_id", event.EventID,
			"error", err.Error(),
		)
		_ = d.Nack(false, true) // Requeue for transient DB issues
		return
	}

	if !newlyRecorded {
		logger.FromContext(ctx).Warn("duplicate event received, skipping fulfillment",
			"event_id", event.EventID,
			"event_type", event.EventType,
			"transaction_id", event.TransactionID,
		)
		_ = d.Ack(false)
		metrics.EventsProcessedTotal.WithLabelValues(event.EventType, "duplicate").Inc()
		return
	}

	// Process business action (item fulfillment simulation)
	switch event.EventType {
	case domain.EventPaymentSucceeded:
		w.fulfillPurchase(ctx, &event)
	case domain.EventPaymentRefunded:
		w.processRefund(ctx, &event)
	default:
		logger.FromContext(ctx).Warn("unhandled event type", "type", event.EventType)
	}

	_ = d.Ack(false)
	metrics.EventsProcessedTotal.WithLabelValues(event.EventType, "success").Inc()
}

func (w *Worker) fulfillPurchase(ctx context.Context, event *domain.PaymentEvent) {
	// Simulate in-game item or game currency grant
	logger.FromContext(ctx).Info("FULFILLMENT: Granted in-game item to player",
		"event_id", event.EventID,
		"player_id", event.PlayerID,
		"wallet_id", event.WalletID,
		"transaction_id", event.TransactionID,
		"amount", event.Amount,
		"currency", event.Currency,
		"metadata", string(event.Metadata),
		"fulfilled_at", time.Now().UTC().Format(time.RFC3339),
	)
}

func (w *Worker) processRefund(ctx context.Context, event *domain.PaymentEvent) {
	// Simulate revoking or adjusting player's inventory
	logger.FromContext(ctx).Info("FULFILLMENT: Revoked in-game item for refunded transaction",
		"event_id", event.EventID,
		"player_id", event.PlayerID,
		"wallet_id", event.WalletID,
		"transaction_id", event.TransactionID,
		"amount", event.Amount,
		"currency", event.Currency,
		"reference_id", event.ReferenceID,
		"revoked_at", time.Now().UTC().Format(time.RFC3339),
	)
}
