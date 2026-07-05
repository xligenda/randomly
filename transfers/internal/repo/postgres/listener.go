package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/lib/pq"
)

const pingDelay = time.Second * 90

type EventListener struct {
	connStr string
}

func NewEventListener(connStr string) *EventListener {
	return &EventListener{connStr: connStr}
}

func (l *EventListener) Listen(ctx context.Context, channel string, wakeUp chan<- struct{}) error {
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Printf("pq listener error: %v\n", err)
		}
	}

	listener := pq.NewListener(l.connStr, 10*time.Second, time.Minute, reportProblem)
	defer listener.Close()

	if err := listener.Listen(channel); err != nil {
		return fmt.Errorf("listen on channel %s: %w", channel, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case n := <-listener.Notify:
			if n == nil {
				continue
			}
			select {
			case wakeUp <- struct{}{}:
			default:
			}

		case <-time.After(pingDelay):
			go listener.Ping()
		}
	}
}
