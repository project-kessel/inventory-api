package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Driver interface {
	Close(ctx context.Context) error
	Connect(ctx context.Context) error
	Listen(ctx context.Context) error
	Ping(ctx context.Context) error
	WaitForNotification(ctx context.Context) (*Notification, error)
	Notify(ctx context.Context, payload string) error
}

func NewPgxDriver(dbp *pgxpool.Pool) Driver {
	return &pgxdriver{
		mu:     sync.Mutex{},
		dbPool: dbp,
	}
}

type pgxdriver struct {
	conn   *pgxpool.Conn
	dbPool *pgxpool.Pool
	mu     sync.Mutex
}

func (d *pgxdriver) Close(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn == nil {
		return nil
	}
	err := d.conn.Conn().Close(ctx)
	d.conn.Release()
	d.conn = nil

	return err
}

func (d *pgxdriver) Connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn != nil {
		return errors.New("connection already established")
	}

	conn, err := d.dbPool.Acquire(ctx)
	if err != nil {
		return err
	}

	d.conn = conn
	return nil
}

func (d *pgxdriver) Listen(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(ctx, "listen consumer_notifications")
	return err
}

func (d *pgxdriver) Ping(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.Ping(ctx)
}

func (d *pgxdriver) WaitForNotification(ctx context.Context) (*Notification, error) {
	if err := d.ensureConnected(ctx); err != nil {
		return nil, fmt.Errorf("WaitForNotification: error re-establishing connection: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	pgn, err := d.conn.Conn().WaitForNotification(ctx)

	if err != nil {
		return nil, err
	}

	n := Notification{
		Channel: pgn.Channel,
		Payload: []byte(pgn.Payload),
	}

	return &n, nil
}

func (d *pgxdriver) Notify(ctx context.Context, payload string) error {
	// If the connection has been lost for some reason, re-establish it
	if err := d.ensureConnected(ctx); err != nil {
		return fmt.Errorf("Notify: error re-establishing connection: %w", err)
	}
	_, err := d.conn.Exec(ctx, `select pg_notify('consumer_notifications', $1)`, payload)
	return err
}

func (d *pgxdriver) ensureConnected(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn == nil || d.conn.Conn().IsClosed() {
		if d.conn != nil {
			d.conn.Release()
			d.conn = nil
		}
		if err := d.Connect(ctx); err != nil {
			return err
		}
	}
	return nil
}

const (
	mockChannel = "mock_channel"
	mockPayload = "mock_payload"
)

type DriverMock struct{}

var _ Driver = &DriverMock{}

func (d *DriverMock) Close(ctx context.Context) error {
	return nil
}
func (d *DriverMock) Connect(ctx context.Context) error {
	return nil
}
func (d *DriverMock) Listen(ctx context.Context) error {
	return nil
}
func (d *DriverMock) Ping(ctx context.Context) error {
	return nil
}
func (d *DriverMock) WaitForNotification(ctx context.Context) (*Notification, error) {
	notif := &Notification{
		Channel: mockChannel,
		Payload: []byte(mockPayload),
	}
	return notif, nil
}
func (d *DriverMock) Notify(ctx context.Context, payload string) error {
	return nil
}
