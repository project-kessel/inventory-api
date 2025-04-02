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

func NewDriver(dbp *pgxpool.Pool) Driver {
	return &driver{
		mu:     sync.Mutex{},
		dbPool: dbp,
	}
}

type driver struct {
	conn   *pgxpool.Conn
	dbPool *pgxpool.Pool
	mu     sync.Mutex
}

func (d *driver) Close(ctx context.Context) error {
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

func (d *driver) Connect(ctx context.Context) error {
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

func (d *driver) Listen(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(ctx, "listen consumer_notifications")
	return err
}

func (d *driver) Ping(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.Ping(ctx)
}

func (d *driver) WaitForNotification(ctx context.Context) (*Notification, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If the connection has been lost for some reason, re-establish it
	if d.conn == nil || d.conn.Conn().IsClosed() {
		d.conn.Release()
		d.conn = nil

		if err := d.Connect(ctx); err != nil {
			return nil, fmt.Errorf("waitForNotification: error re-establishing pgx connection: %w", err)
		}
	}

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

func (d *driver) Notify(ctx context.Context, payload string) error {
	// If the connection has been lost for some reason, re-establish it
	if d.conn == nil || d.conn.Conn().IsClosed() {
		d.conn.Release()
		d.conn = nil

		if err := d.Connect(ctx); err != nil {
			return fmt.Errorf("notify: error re-establishing pgx connection: %w", err)
		}
	}
	_, err := d.conn.Exec(ctx, `select pg_notify('consumer_notifications', $1)`, payload)
	return err
}
