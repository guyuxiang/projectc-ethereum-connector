package rabbitmq

import (
	"errors"
	"sync"
	"time"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/log"
	"github.com/streadway/amqp"
)

var (
	mu                sync.RWMutex
	conn              *amqp.Connection
	channel           *amqp.Channel
	cfg               *config.RabbitMQ
	stopCh            chan struct{}
	reconnectInterval = 5 * time.Second
)

func Init(rabbitCfg *config.RabbitMQ) (*amqp.Channel, error) {
	if rabbitCfg == nil {
		return nil, errors.New("rabbitmq config is nil")
	}
	if rabbitCfg.URL == "" {
		return nil, errors.New("rabbitmq url is empty")
	}

	normalized := normalizeConfig(rabbitCfg)

	if err := Close(); err != nil {
		return nil, err
	}

	if err := connect(normalized); err != nil {
		return nil, err
	}

	mu.Lock()
	cfg = normalized
	stopCh = make(chan struct{})
	currentChannel := channel
	mu.Unlock()

	go watchClose()

	return currentChannel, nil
}

func Connection() *amqp.Connection {
	mu.RLock()
	defer mu.RUnlock()

	return conn
}

func Channel() *amqp.Channel {
	mu.RLock()
	defer mu.RUnlock()

	return channel
}

func Publish(body []byte) error {
	mu.RLock()
	localConn := conn
	localCfg := cfg
	mu.RUnlock()

	if localConn == nil || localCfg == nil {
		return errors.New("rabbitmq is not initialized")
	}

	pubChannel, err := localConn.Channel()
	if err != nil {
		return err
	}
	defer pubChannel.Close()

	if err = declareTopology(pubChannel, localCfg); err != nil {
		return err
	}

	return pubChannel.Publish(
		localCfg.Exchange,
		localCfg.RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/octet-stream",
			Body:        body,
		},
	)
}

func Consume(consumer string, autoAck bool) (<-chan amqp.Delivery, func() error, error) {
	mu.RLock()
	localConn := conn
	localCfg := cfg
	mu.RUnlock()

	if localConn == nil || localCfg == nil {
		return nil, nil, errors.New("rabbitmq is not initialized")
	}
	if localCfg.Queue == "" {
		return nil, nil, errors.New("rabbitmq queue is empty")
	}

	consumeChannel, err := localConn.Channel()
	if err != nil {
		return nil, nil, err
	}

	if err = declareTopology(consumeChannel, localCfg); err != nil {
		_ = consumeChannel.Close()
		return nil, nil, err
	}

	deliveries, err := consumeChannel.Consume(
		localCfg.Queue,
		consumer,
		autoAck,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = consumeChannel.Close()
		return nil, nil, err
	}

	closeFn := func() error {
		return consumeChannel.Close()
	}

	return deliveries, closeFn, nil
}

func Close() error {
	mu.Lock()
	localStopCh := stopCh
	localChannel := channel
	localConn := conn

	stopCh = nil
	channel = nil
	conn = nil
	cfg = nil
	mu.Unlock()

	if localStopCh != nil {
		close(localStopCh)
	}

	var firstErr error
	if localChannel != nil {
		if err := localChannel.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if localConn != nil {
		if err := localConn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func connect(rabbitCfg *config.RabbitMQ) error {
	newConn, err := amqp.Dial(rabbitCfg.URL)
	if err != nil {
		return err
	}

	newChannel, err := newConn.Channel()
	if err != nil {
		_ = newConn.Close()
		return err
	}

	if err = declareTopology(newChannel, rabbitCfg); err != nil {
		_ = newChannel.Close()
		_ = newConn.Close()
		return err
	}

	mu.Lock()
	oldChannel := channel
	oldConn := conn
	channel = newChannel
	conn = newConn
	mu.Unlock()

	if oldChannel != nil {
		_ = oldChannel.Close()
	}
	if oldConn != nil {
		_ = oldConn.Close()
	}

	return nil
}

func declareTopology(ch *amqp.Channel, rabbitCfg *config.RabbitMQ) error {
	if rabbitCfg.Exchange != "" {
		if err := ch.ExchangeDeclare(
			rabbitCfg.Exchange,
			rabbitCfg.ExchangeType,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return err
		}
	}

	if rabbitCfg.Queue == "" {
		return nil
	}

	if _, err := ch.QueueDeclare(
		rabbitCfg.Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	if rabbitCfg.Exchange == "" {
		return nil
	}

	return ch.QueueBind(
		rabbitCfg.Queue,
		rabbitCfg.RoutingKey,
		rabbitCfg.Exchange,
		false,
		nil,
	)
}

func watchClose() {
	for {
		mu.RLock()
		localConn := conn
		localChannel := channel
		localCfg := cfg
		localStopCh := stopCh
		mu.RUnlock()

		if localConn == nil || localChannel == nil || localCfg == nil || localStopCh == nil {
			return
		}

		connCloseCh := localConn.NotifyClose(make(chan *amqp.Error, 1))
		channelCloseCh := localChannel.NotifyClose(make(chan *amqp.Error, 1))

		var closeErr *amqp.Error
		select {
		case <-localStopCh:
			return
		case closeErr = <-connCloseCh:
		case closeErr = <-channelCloseCh:
		}

		if closeErr == nil {
			return
		}

		log.Warningf("rabbitmq connection closed, reconnecting: %v", closeErr)

		for {
			select {
			case <-localStopCh:
				return
			case <-time.After(reconnectInterval):
			}

			if err := connect(localCfg); err != nil {
				log.Warningf("rabbitmq reconnect failed: %v", err)
				continue
			}

			log.Infof("rabbitmq reconnected successfully")
			break
		}
	}
}

func normalizeConfig(rabbitCfg *config.RabbitMQ) *config.RabbitMQ {
	normalized := *rabbitCfg
	if normalized.ExchangeType == "" {
		normalized.ExchangeType = "direct"
	}
	if normalized.RoutingKey == "" {
		normalized.RoutingKey = normalized.Queue
	}

	return &normalized
}
