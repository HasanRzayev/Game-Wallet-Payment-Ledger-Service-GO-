package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"game-wallet-ledger/internal/config"
	"game-wallet-ledger/pkg/logger"
)

type Broker struct {
	cfg     config.RabbitMQConfig
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
}

func NewBroker(ctx context.Context, cfg config.RabbitMQConfig) (*Broker, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}

	// Declare durable topic exchange
	err = ch.ExchangeDeclare(
		cfg.ExchangeName, // name
		"topic",          // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare rabbitmq exchange %s: %w", cfg.ExchangeName, err)
	}

	// Declare durable queue
	q, err := ch.QueueDeclare(
		cfg.QueueName, // name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare rabbitmq queue %s: %w", cfg.QueueName, err)
	}

	// Bind queue to topic exchange
	err = ch.QueueBind(
		q.Name,           // queue name
		cfg.RoutingKey,   // routing key (e.g. payment.*)
		cfg.ExchangeName, // exchange
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to bind queue %s to exchange %s: %w", q.Name, cfg.ExchangeName, err)
	}

	logger.FromContext(ctx).Info("connected to rabbitmq and initialized exchange & queue",
		"exchange", cfg.ExchangeName,
		"queue", cfg.QueueName,
		"routing_key", cfg.RoutingKey,
	)

	return &Broker{
		cfg:     cfg,
		conn:    conn,
		channel: ch,
	}, nil
}

func (b *Broker) Channel() *amqp.Channel {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.channel
}

func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var err error
	if b.channel != nil {
		if chErr := b.channel.Close(); chErr != nil {
			err = chErr
		}
	}
	if b.conn != nil {
		if connErr := b.conn.Close(); connErr != nil && err == nil {
			err = connErr
		}
	}
	return err
}
