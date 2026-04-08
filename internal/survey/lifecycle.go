package survey

import (
	"context"
	"log"
	"time"
)

type AutoCloser struct {
	store    *Store
	interval time.Duration
	stop     chan struct{}
}

func NewAutoCloser(store *Store, interval time.Duration) *AutoCloser {
	return &AutoCloser{
		store:    store,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (ac *AutoCloser) Start() {
	go func() {
		ticker := time.NewTicker(ac.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := ac.store.CloseExpired(context.Background())
				if err != nil {
					log.Printf("auto-close error: %v", err)
				} else if n > 0 {
					log.Printf("auto-closed %d survey(s)", n)
				}
			case <-ac.stop:
				return
			}
		}
	}()
}

func (ac *AutoCloser) Stop() {
	close(ac.stop)
}
