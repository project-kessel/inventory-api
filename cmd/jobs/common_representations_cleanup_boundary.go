package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"gorm.io/gorm"
)

// commonRepCleanupBoundary reads existing progress; it never produces messages,
// changes offsets, or pauses writers. Slot acknowledgment is valid only for the
// configured Debezium connector with durable source-offset commits (see runbook).
type commonRepCleanupBoundary struct {
	db    *gorm.DB
	admin *kafka.AdminClient
	topic string
	group string
	slot  string
	log   *log.Helper
}

func newCommonRepCleanupBoundary(db *gorm.DB, options *consumer.Options, slot string, logger *log.Helper) (*commonRepCleanupBoundary, error) {
	if slot == "" {
		return nil, fmt.Errorf("--replication-slot is required for cleanup execution")
	}
	if options == nil || !options.Enabled || options.EnableAutoCommit != "false" || options.Topic == "" || options.ConsumerGroupID == "" {
		return nil, fmt.Errorf("cleanup requires an enabled Inventory consumer with explicit offset commits, topic and group")
	}
	config, errs := consumer.NewConfig(options).Complete()
	if len(errs) != 0 {
		return nil, fmt.Errorf("configure cleanup Kafka access: %v", errs)
	}
	// Reuse the actual consumer's broker, authentication and TLS configuration.
	// AdminClient does not join or advance the consumer group.
	admin, err := kafka.NewAdminClient(config.KafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("open cleanup Kafka admin client: %w", err)
	}
	return &commonRepCleanupBoundary{db: db, admin: admin, topic: options.Topic, group: options.ConsumerGroupID, slot: slot, log: logger}, nil
}

type commonRepCleanupSourcePosition struct {
	StartedAt time.Time
	Recovery  bool
	Active    bool
	Plugin    string
	Reached   bool
}

func (boundary *commonRepCleanupBoundary) sourcePosition(ctx context.Context, wal string) (commonRepCleanupSourcePosition, error) {
	var position commonRepCleanupSourcePosition
	result := boundary.db.WithContext(ctx).Raw(`
		SELECT pg_postmaster_start_time() AS started_at, pg_is_in_recovery() AS recovery,
		       active, plugin, COALESCE(confirmed_flush_lsn >= ?::pg_lsn, false) AS reached
		FROM pg_replication_slots WHERE slot_name = ? AND database = current_database()`, wal, boundary.slot).Scan(&position)
	if result.Error != nil {
		return position, fmt.Errorf("read cleanup replication slot: %w", result.Error)
	}
	if result.RowsAffected != 1 || position.Recovery || position.Plugin != "pgoutput" {
		return position, fmt.Errorf("cleanup requires the configured pgoutput slot on this primary database")
	}
	return position, nil
}

func (boundary *commonRepCleanupBoundary) topicID(ctx context.Context) (string, error) {
	result, err := boundary.admin.DescribeTopics(ctx, kafka.NewTopicCollectionOfTopicNames([]string{boundary.topic}))
	if err != nil {
		return "", fmt.Errorf("describe cleanup topic: %w", err)
	}
	if len(result.TopicDescriptions) != 1 {
		return "", fmt.Errorf("cleanup topic metadata is missing")
	}
	topic := result.TopicDescriptions[0]
	if topic.Error.Code() != kafka.ErrNoError {
		return "", fmt.Errorf("describe cleanup topic: %w", topic.Error)
	}
	// The Inventory consumer currently fences one assigned partition. Refuse to
	// infer a broader consumption guarantee if the deployment changes that shape.
	if len(topic.Partitions) != 1 || topic.Partitions[0].Partition != 0 ||
		(topic.TopicID.GetMostSignificantBits() == 0 && topic.TopicID.GetLeastSignificantBits() == 0) {
		return "", fmt.Errorf("cleanup requires an identifiable, single-partition tuple topic")
	}
	return topic.TopicID.String(), nil
}

func (boundary *commonRepCleanupBoundary) offset(ctx context.Context, specification kafka.OffsetSpec) (kafka.Offset, error) {
	partition := kafka.TopicPartition{Topic: &boundary.topic, Partition: 0}
	result, err := boundary.admin.ListOffsets(ctx, map[kafka.TopicPartition]kafka.OffsetSpec{partition: specification})
	if err != nil {
		return kafka.OffsetInvalid, fmt.Errorf("read cleanup topic offset: %w", err)
	}
	for key, value := range result.ResultInfos {
		if key.Topic != nil && *key.Topic == boundary.topic && key.Partition == 0 {
			if value.Error.Code() != kafka.ErrNoError {
				return kafka.OffsetInvalid, fmt.Errorf("read cleanup topic offset: %w", value.Error)
			}
			if value.Offset >= 0 {
				return value.Offset, nil
			}
		}
	}
	return kafka.OffsetInvalid, fmt.Errorf("cleanup topic offset is missing")
}

func (boundary *commonRepCleanupBoundary) committed(ctx context.Context) (kafka.Offset, error) {
	result, err := boundary.admin.ListConsumerGroupOffsets(ctx, []kafka.ConsumerGroupTopicPartitions{{
		Group: boundary.group, Partitions: []kafka.TopicPartition{{Topic: &boundary.topic, Partition: 0}},
	}})
	if err != nil {
		return kafka.OffsetInvalid, fmt.Errorf("read Inventory committed offset: %w", err)
	}
	for _, group := range result.ConsumerGroupsTopicPartitions {
		if group.Group == boundary.group {
			for _, partition := range group.Partitions {
				if partition.Topic != nil && *partition.Topic == boundary.topic && partition.Partition == 0 {
					if partition.Error != nil {
						return kafka.OffsetInvalid, fmt.Errorf("read Inventory committed offset: %w", partition.Error)
					}
					if partition.Offset >= 0 {
						return partition.Offset, nil
					}
				}
			}
		}
	}
	return kafka.OffsetInvalid, fmt.Errorf("inventory committed offset is missing; refusing cleanup")
}

// Await is called after chunk capture commits. Source acknowledgment must precede
// sampling Kafka's end offset: sampling earlier can miss events still inside CDC.
// Both targets remain fixed while writes continue. The returned validator checks
// the certificate again before each deletion transaction.
func (boundary *commonRepCleanupBoundary) Await(ctx context.Context) (func(context.Context) error, error) {
	var wal string
	if err := boundary.db.WithContext(ctx).Raw("SELECT pg_current_wal_insert_lsn()::text").Scan(&wal).Error; err != nil {
		return nil, fmt.Errorf("capture cleanup WAL boundary: %w", err)
	}
	initialSource, err := boundary.sourcePosition(ctx, wal)
	if err != nil {
		return nil, err
	}
	initialTopic, err := boundary.topicID(ctx)
	if err != nil {
		return nil, err
	}
	// Reject missing offsets or an existing retention gap before waiting. A later
	// automatic reset must not turn discarded events into apparent consumption.
	checkConsumer := func(ctx context.Context) (kafka.Offset, error) {
		topic, err := boundary.topicID(ctx)
		if err != nil {
			return kafka.OffsetInvalid, err
		}
		if topic != initialTopic {
			return kafka.OffsetInvalid, fmt.Errorf("cleanup tuple topic was recreated")
		}
		committed, err := boundary.committed(ctx)
		if err != nil {
			return kafka.OffsetInvalid, err
		}
		earliest, err := boundary.offset(ctx, kafka.EarliestOffsetSpec)
		if err != nil {
			return kafka.OffsetInvalid, err
		}
		if committed < earliest {
			return kafka.OffsetInvalid, fmt.Errorf("inventory committed offset is behind retained Kafka history")
		}
		return committed, nil
	}
	if _, err := checkConsumer(ctx); err != nil {
		return nil, err
	}
	checkSource := func(ctx context.Context) (bool, error) {
		position, err := boundary.sourcePosition(ctx, wal)
		if err != nil {
			return false, err
		}
		if !position.StartedAt.Equal(initialSource.StartedAt) {
			return false, fmt.Errorf("database restarted during cleanup boundary verification")
		}
		return position.Active && position.Reached, nil
	}
	boundary.log.Infow("msg", "waiting for cleanup CDC boundary", "wal_boundary", wal, "replication_slot", boundary.slot)
	for {
		reached, err := checkSource(ctx)
		if err != nil {
			return nil, err
		}
		if reached {
			break
		}
		if _, err := checkConsumer(ctx); err != nil {
			return nil, err
		}
		if err := waitForCommonRepNextBatch(ctx, time.Second); err != nil {
			return nil, err
		}
	}
	target, err := boundary.offset(ctx, kafka.LatestOffsetSpec)
	if err != nil {
		return nil, err
	}
	boundary.log.Infow("msg", "waiting for cleanup consumer boundary", "wal_boundary", wal, "consumer_group", boundary.group, "target_offset", int64(target))
	for {
		committed, err := checkConsumer(ctx)
		if err != nil {
			return nil, err
		}
		// The broker's committed offset is a restart position. Do not subtract
		// one: replaying the final pre-boundary record could need deleted history.
		if committed >= target {
			break
		}
		if err := waitForCommonRepNextBatch(ctx, time.Second); err != nil {
			return nil, err
		}
	}
	boundary.log.Infow("msg", "cleanup consumption boundary verified", "wal_boundary", wal, "target_offset", int64(target))
	return func(ctx context.Context) error {
		reached, err := checkSource(ctx)
		if err != nil {
			return err
		}
		if !reached {
			return fmt.Errorf("cleanup source checkpoint is no longer valid")
		}
		committed, err := checkConsumer(ctx)
		if err != nil {
			return err
		}
		if committed < target {
			return fmt.Errorf("inventory consumer offset moved behind the cleanup boundary")
		}
		return nil
	}, nil
}
