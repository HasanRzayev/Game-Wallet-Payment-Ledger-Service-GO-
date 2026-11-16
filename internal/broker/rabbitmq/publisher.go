package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"game-wallet-ledger/internal/domain"
	"game-wallet-ledger/pkg/logger"
)

type EventPublisher interface {
	PublishPaymentEvent(ctx context.Context, event *domain.PaymentEvent) error
}

type Publisher struct {
	broker *Broker
}

func NewPublisher(broker *Broker) *Publisher {
	return &Publisher{broker: broker}
}

func (p *Publisher) PublishPaymentEvent(ctx context.Context, event *domain.PaymentEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal payment event: %w", err)
	}

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		MessageId:    event.EventID,
		Timestamp:    event.Timestamp,
		Body:         body,
		Headers: amqp.Table{
			"event_type": event.EventType,
		},
	}

	// Topic routing key matches event type, e.g., "payment.succeeded" or "payment.refunded"
	routingKey := event.EventType

	ch := p.broker.Channel()
	err = ch.PublishWithContext(
		ctx,
		p.broker.cfg.ExchangeName, // exchange
		routingKey,                // routing key
		false,                     // mandatory
		false,                     // immediate
		msg,
	)
	if err != nil {
		return fmt.Errorf("failed to publish rabbitmq event %s: %w", event.EventID, err)
	}

	logger.FromContext(ctx).Info("published payment event to rabbitmq",
		"event_id", event.EventID,
		"event_type", event.EventType,
		"transaction_id", event.TransactionID,
		"wallet_id", event.WalletID,
	)

	return nil
}
