/*

package dbnotify provides a backchannel from the database to push changes to
models out to other locations.

TODO: Deleted notifications.  Gossip supports them, we don't.  Bug.
*/

package dbnotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	sleepOnErrorTime = 5 * time.Second
)

type NotificationEvent struct {
	Table   string
	OnID    int64
	Version int64
}

type DBNotifyListener struct {
	db                  *sql.DB
	tableNameToConsumer map[string]Consumer
}

type CacheStorage[StoredType any] interface {
	CacheInvalidate(ctx context.Context, key int64, version int64)
}

// Probably implemented by ChangeDispatcher, but this version doesn't see the
// generic.
type ChangeConsumer interface {
	Consume(event *NotificationEvent)
}

// Caller must implement.
type ClientNotifier[StoredType any] interface {
	NotifyUpdated(ctx context.Context, m StoredType)
	// TODO: Add and require NotifyDeleted
}

type StorageFetcher[StoredType any] interface {
	Fetch(ctx context.Context, id int64) (StoredType, error)
}

// Caller may use.  This will provide an implementation of consume,
// if the other things are passed in.
type ChangeDispatcher[StoredType any] struct {
	tableName      string
	clientNotifier ClientNotifier[StoredType]
	cacheStorage   CacheStorage[StoredType]
	fetcher        StorageFetcher[StoredType]
}

func (cd *ChangeDispatcher[StoredType]) TableName() string {
	return cd.tableName
}

func NewChangeDispatcher[StoredType any](tableName string, clientNotifier ClientNotifier[StoredType], cacheStorage CacheStorage[StoredType], fetcher StorageFetcher[StoredType]) *ChangeDispatcher[StoredType] {
	return &ChangeDispatcher[StoredType]{
		tableName:      tableName,
		clientNotifier: clientNotifier,
		cacheStorage:   cacheStorage,
		fetcher:        fetcher,
	}
}

type Consumer interface {
	TableName() string
	Consume(ctx context.Context, event *NotificationEvent)
}

func NewDBNotifyListener(db *sql.DB, consumers ...Consumer) (*DBNotifyListener, error) {
	m := make(map[string]Consumer)
	for _, c := range consumers {
		tableName := c.TableName()
		if _, exists := m[tableName]; exists {
			return nil, fmt.Errorf("duplicate consumer for table %s", tableName)
		}
		m[tableName] = c
	}

	return &DBNotifyListener{db: db, tableNameToConsumer: m}, nil
}

func (cl *DBNotifyListener) Close() error {
	err := cl.db.Close()
	cl.db = nil
	return err
}

func (cl *DBNotifyListener) Listen(ctx context.Context) error {
	conn, err := cl.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	var pgxConn *stdlib.Conn
	err = conn.Raw(func(driverConn any) error {
		pgxConn = driverConn.(*stdlib.Conn)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to get pgx connection: %w", err)
	}

	channels := []string{}
	for table := range cl.tableNameToConsumer {
		channel := fmt.Sprintf("%s_changes", table)
		channels = append(channels, channel)
	}
	for _, channel := range channels {
		_, err := pgxConn.Conn().Exec(ctx, fmt.Sprintf("LISTEN %s", channel))
		if err != nil {
			return fmt.Errorf("failed to listen on channel %s: %w", channel, err)
		}
	}

	ch := make(chan *NotificationEvent)
	defer close(ch)
	go cl.consumeEvents(ctx, ch)

	for {
		log.Printf("(awaiting db notifications...)")
		var notification *pgconn.Notification
		if nf, err := pgxConn.Conn().WaitForNotification(ctx); err == nil {
			notification = nf
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("error waiting for notification: %w", err)
		}

		log.Printf("(received db notification %d %s)", notification.PID, string(notification.Payload))

		event := &NotificationEvent{}
		if err := json.Unmarshal([]byte(notification.Payload), &event); err != nil {
			log.Printf("can't unmarshal notification payload '%s': %v", notification.Payload, err)
			time.Sleep(sleepOnErrorTime)
			continue
		}

		ch <- event
	}
}

func (cd *DBNotifyListener) consumeEvents(ctx context.Context, ch <-chan *NotificationEvent) {
	for {
		select {
		case <-ctx.Done():
			// Unclear if we need this; we are already reading on a channel that can close.
			log.Printf("stopping DBChangeListener consumeEvents due to context done: %v", ctx.Err())
			return
		case event := <-ch:
			log.Printf("received db notification event: %+v", event)
			go func() {
				listener, ok := cd.tableNameToConsumer[event.Table]
				if ok {
					listener.Consume(ctx, event)
				} else {
					log.Printf("no listener for table %s", event.Table)
				}
			}()
		}
	}
}

func (cd *ChangeDispatcher[StoredType]) Consume(ctx context.Context, event *NotificationEvent) {
	cd.cacheStorage.CacheInvalidate(ctx, event.OnID, event.Version)

	// Read-through.
	item, err := cd.fetcher.Fetch(ctx, event.OnID)
	if err != nil {
		log.Printf("drop notiication: can't fetch item %s %d: %v", cd.tableName, event.OnID, err)
	}

	if cd.clientNotifier != nil {
		cd.clientNotifier.NotifyUpdated(ctx, item)
	}
}
