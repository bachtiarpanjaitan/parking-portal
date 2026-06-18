// Package worker orchestrates concurrent consumers.
package worker

import (
	"context"
	"log"
	"sync"

	"github.com/parking-portal/backend/notification-worker/internal/consumer"
	"github.com/parking-portal/backend/notification-worker/internal/notifications"
	pkgEvents "github.com/parking-portal/backend/pkg/events"
)

// Run spawns `concurrency` consumer goroutines, each consuming from the
// same AMQP queue. The AMQP broker distributes messages across them.
//
// Blocks until ctx is cancelled, then waits for in-flight handlers and
// returns.
func Run(ctx context.Context, c *consumer.Consumer, svc *notifications.Service, concurrency int) error {
	if concurrency < 1 {
		concurrency = 1
	}
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			log.Printf("[worker-%d] starting", id)
			if err := c.Run(ctx, func(c context.Context, env *pkgEvents.Envelope) error {
				return svc.Handle(c, env)
			}); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
