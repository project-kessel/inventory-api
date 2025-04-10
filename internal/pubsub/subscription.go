package pubsub

import (
	"context"
	"fmt"
	"sync"
)

type Subscription interface {
	NotificationC() <-chan []byte
	Unsubscribe()
	BlockForNotification(ctx context.Context) error
}

type subscription struct {
	txId          string
	listenChan    chan []byte
	listenManager *ListenManager
	unsubOnce     sync.Once
}

func (s *subscription) NotificationC() <-chan []byte { return s.listenChan }

func (s *subscription) Unsubscribe() {
	s.unsubOnce.Do(func() {
		// Unlisten uses background context in case of cancellation.
		if err := s.listenManager.unsubscribe(context.Background(), s); err != nil {
			s.listenManager.logger.Error("error unlistening on channel", "err", err, "txId", s.txId)
		}
	})
}

func (s *subscription) BlockForNotification(ctx context.Context) error {
	s.listenManager.logger.Debugf("Blocking for notification: %v", s.txId)
	for {
		select {
		case notification := <-s.NotificationC():
			if string(notification) == s.txId {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for notification")
		}
	}
}
