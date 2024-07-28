package kafka

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"

	confluent "github.com/cloudevents/sdk-go/protocol/kafka_confluent/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/models"
)

type KafkaManager struct {
	Config   CompletedConfig
	Protocol *confluent.Protocol
	Client   cloudevents.Client
	Errors   <-chan error
}

func New(config CompletedConfig, logger log.Logger) (*KafkaManager, error) {
	if sender, err := confluent.New(
		confluent.WithSenderTopic(config.DefaultTopic),
		confluent.WithConfigMap(config.KafkaConfig),
	); err != nil {
		return nil, err
	} else {
		client, err := cloudevents.NewClient(sender, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
		if err != nil {
			return nil, err
		}

		errChan := make(chan error)

		go func() {
			eventChan, err := sender.Events()
			if err != nil {
				logger.Log(log.LevelError, "msg", fmt.Sprintf("failed to get events channel for sender, %v", err))
				errChan <- err
			} else {
				for e := range eventChan {
					switch ev := e.(type) {
					case *kafka.Message:
						// The message delivery report, indicating success or permanent failure after retries have
						// been exhausted. Application level retries won't help since the client is already
						// configured to do that.
						m := ev
						if m.TopicPartition.Error != nil {
							logger.Log(log.LevelError, "msg", fmt.Sprintf("Delivery failed: %v\n", m.TopicPartition.Error))
							errChan <- err
						} else {
							logger.Log(log.LevelInfo, "msg", fmt.Sprintf("Delivered message to topic %s [%d] at offset %v\n",
								*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset))
						}
					case kafka.Error:
						e := ev
						if e.IsFatal() {
							logger.Log(log.LevelError, "msg", fmt.Sprintf("Error: %v\n", ev))
							errChan <- e
						} else {
							logger.Log(log.LevelInfo, "msg", fmt.Sprintf("Error: %v\n", ev))
						}
					default:
						logger.Log(log.LevelInfo, "msg", fmt.Sprintf("Ignored event: %v\n", ev))
					}
				}
			}
		}()

		return &KafkaManager{
			Config:   config,
			Protocol: sender,
			Client:   client,
			Errors:   errChan,
		}, nil
	}
}

func (m *KafkaManager) Errs() <-chan error {
	return m.Errors
}

// Lookup figures out which topic should be used for the given identity and resource.
func (m *KafkaManager) Lookup(identity *authnapi.Identity, resource *models.Resource) (api.Producer, error) {

	// there is no complicated topic dispatch logic... for now.
	return NewProducer(m, m.Config.DefaultTopic, identity), nil
}

func (m *KafkaManager) Shutdown(ctx context.Context) error {
	return m.Protocol.Close(ctx)
}

type kafkaProducer struct {
	Manager  *KafkaManager
	Topic    string
	Identity *authnapi.Identity
}

// NewProducer produces a kafka producer that is bound to a particular topic.
func NewProducer(manager *KafkaManager, topic string, identity *authnapi.Identity) *kafkaProducer {
	return &kafkaProducer{
		Manager:  manager,
		Topic:    topic,
		Identity: identity,
	}
}

// Produce creates the cloud event and sends it on the Kafka Topic
func (p *kafkaProducer) Produce(ctx context.Context, event *api.Event) error {
	e := cloudevents.NewEvent()
	e.SetData(cloudevents.ApplicationJSON, event)
	return p.Manager.Client.Send(cecontext.WithTopic(ctx, p.Topic), e)
}
