package e2e

import (
	"context"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/xeipuuv/gojsonschema"
	"os"
	"testing"
	"time"
)

// JSONSchema for Inventory Event Structure
const inventoryEventSchema = `{
	"title": "Inventory Event Structure",
	"description": "The event schema will be compatible with CloudEvents, a specification for describing event data in a common way. The following describes how the fabric will align with the CloudEvent schema.",
	"type": "object",
	"properties": {
		"specversion": {
			"description": "Specifies the version of the CloudEvents spec targeted.",
			"type": "string",
			"enum": ["1.0"]
		},
		"type": {
			"description": "We use a string comprised of redhat.inventory.(resources|resources_relationship).{resource_type}.(created|updated|deleted)",
			"type": "string",
			"pattern": "^redhat\\.inventory\\.(resources|resources_relationship)\\.[a-zA-Z0-9_-]+\\.(created|updated|deleted)$",
			"examples": [
				"redhat.inventory.resources.k8s_cluster.created",
				"redhat.inventory.resources.k8s_cluster.updated",
				"redhat.inventory.resources.k8s_cluster.deleted",
				"redhat.inventory.resources_relationship.k8spolicy_ispropagatedto_k8scluster.created",
				"redhat.inventory.resources_relationship.k8spolicy_ispropagatedto_k8scluster.updated",
				"redhat.inventory.resources_relationship.k8spolicy_ispropagatedto_k8scluster.deleted"
			]
		},
		"source": {
			"description": "Describes the source (or app) that generated the event.",
			"type": "string",
			"format": "uri",
			"examples": ["https://redhat.com"]
		},
		"id": {
			"description": "Identifies the event. Unique for this source.",
			"type": "string",
			"format": "uuid",
			"examples": ["afebabe-cafe-babe-cafe-babecafebabe"]
		},
		"time": {
			"description": "Last reported from inventory-api",
			"type": "string",
			"format": "date-time",
			"examples": ["2018-11-13T20:20:39+00:00"]
		},
		"datacontenttype": {
			"description": "Content type of data value",
			"type": "string",
			"pattern": "^application\\/json$"
		},
		"data": {
			"type": "object"
		},
		"subject": {
			"description": "Represents the updated resource: (resource|resources-relation)/{resource_type}/{resource_id}",
			"type": "string",
			"pattern": "^\\/(resources|resources-relationships)\\/[a-zA-Z0-9_-]+\\/[a-zA-Z0-9-]+$",
			"examples": [
				"/resources/k8s_cluster/A234-1234-1234",
				"/resources-relationships/k8spolicy_ispropagatedto_k8scluster/A234-1234-1234"
			]
		}
	},
	"required": ["specversion", "type", "source", "id", "time", "datacontenttype", "data", "subject"]
}`

func getEnvOrDefault(envVar, defaultValue string) string {
	val := os.Getenv(envVar)
	if val == "" {
		return defaultValue
	}
	return val
}

// Test_ACMKafkaConsumer reads events from a Kafka topic and verifies their schema.
func Test_ACMKafkaConsumer(t *testing.T) {
	t.Parallel()
	kafkaBootstrapServers := getEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
	kafkaSecProtocol := os.Getenv("KAFKA_SECURITY_PROTOCOL")
	kafkaCaLocation := os.Getenv("KAFKA_SSL_CA_LOCATION")
	kafkaCertLocation := os.Getenv("KAFKA_SSL_CERT_LOCATION")
	kafkaKeyLocation := os.Getenv("KAFKA_SSL_KEY_LOCATION")
	topic := getEnvOrDefault("KAFKA_TOPIC", "kessel-inventory")

	config := &kafka.ConfigMap{
		"bootstrap.servers": kafkaBootstrapServers,
		"group.id":          "server",
		"auto.offset.reset": "earliest",
	}

	if kafkaSecProtocol != "" {
		if kafkaCaLocation == "" || kafkaCertLocation == "" || kafkaKeyLocation == "" {
			log.Fatalf("SSL configuration is incomplete. Please provide KAFKA_SSL_CA_LOCATION, KAFKA_SSL_CERT_LOCATION, and KAFKA_SSL_KEY_LOCATION.")
		}
		err := config.SetKey("security.protocol", kafkaSecProtocol)
		if err != nil {
			err = fmt.Errorf("please provide KAFKA_SECURITY_PROTOCOL to set security.protocol")
			log.Error(err)
		}
		err = config.SetKey("ssl.ca.location", kafkaCaLocation)
		if err != nil {
			err = fmt.Errorf("please provide KAFKA_SSL_CA_LOCATION to set ssl.ca.location")
			log.Error(err)
		}
		err = config.SetKey("ssl.certificate.location", kafkaCertLocation)
		if err != nil {
			err = fmt.Errorf("please provide KAFKA_SSL_CERT_LOCATION to set ssl.certificate.location")
			log.Error(err)
		}
		err = config.SetKey("ssl.key.location", kafkaKeyLocation)
		if err != nil {
			err = fmt.Errorf("please provide KAFKA_SSL_KEY_LOCATION to set ssl.key.location")
			log.Error(err)
		}
	}

	consumer, err := kafka.NewConsumer(config)
	if err != nil {
		t.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	defer consumer.Close()

	err = consumer.Subscribe(topic, nil)
	if err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	run := true

	for run {
		select {
		case <-ctx.Done():
			t.Log("Test timed out after 10 minutes of consuming")
			return
		default:
			ev := consumer.Poll(1000)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				// Process the message received.
				fmt.Printf("%% Message on %s:\n%s\n",
					e.TopicPartition, string(e.Value))
				if e.Headers != nil {
					fmt.Printf("%% Headers: %v\n", e.Headers)
				}

				err = VerifyInventoryEventSchema(e.Value, inventoryEventSchema)
				if err != nil {
					t.Errorf("Schema validation failed: %v", err)
				}
				t.Logf("Schema validation passed")
				run = false
			case kafka.Error:
				fmt.Fprintf(os.Stderr, "%% Error: %v: %v\n", e.Code(), e)
				if e.Code() == kafka.ErrAllBrokersDown {
					run = false
				}
			default:
				fmt.Printf("Ignored %v\n", e)
			}
		}
	}
}

func VerifyInventoryEventSchema(jsonMessage []byte, schema string) error {
	// Load the schema
	schemaLoader := gojsonschema.NewStringLoader(schema)

	// Load the message
	documentLoader := gojsonschema.NewBytesLoader(jsonMessage)

	// Validate the message against the schema
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("failed to validate message: %v", err)
	}

	if result.Valid() {
		fmt.Println("The message is valid!")
	} else {
		fmt.Println("The message is invalid:")
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}
	}

	return nil
}
