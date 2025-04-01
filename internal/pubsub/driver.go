package pubsub

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Driver interface {
	Close(ctx context.Context) error
	Connect(ctx context.Context) error
	Listen(ctx context.Context, topic string) error
	Ping(ctx context.Context) error
	Unlisten(ctx context.Context, topic string) error
	WaitForNotification(ctx context.Context) (*Notification, error)
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

func (d *driver) Listen(ctx context.Context, topic string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(ctx, "LISTEN \""+topic+"\"")
	return err
}

func (d *driver) Ping(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.conn.Ping(ctx)
}

func (d *driver) Unlisten(ctx context.Context, topic string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(ctx, "UNLISTEN \""+topic+"\"")
	return err
}

func (d *driver) WaitForNotification(ctx context.Context) (*Notification, error) {
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
