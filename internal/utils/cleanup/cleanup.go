package cleanup

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// CleanupOperation is a clean up function on shutting down
type CleanupOperation func(ctx context.Context) error

// NamedOperation is one shutdown step. Order matters, so the caller supplies a
// slice: these used to run concurrently off a map, which let the gRPC server
// stop while the services still had work in flight.
type NamedOperation struct {
	Name string
	Op   CleanupOperation
}

// GracefulShutdown waits for a termination syscall, then runs ops in the order given.
func GracefulShutdown(ctx context.Context, timeout time.Duration, ops []NamedOperation) <-chan struct{} {
	wait := make(chan struct{})
	go func() {
		s := make(chan os.Signal, 1)

		// add any other syscalls that you want to be notified with
		signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-s

		log.Println("shutting down")

		// set timeout for the ops to be done to prevent system hang
		timeoutFunc := time.AfterFunc(timeout, func() {
			log.Printf("timeout %d ms has been elapsed, force exit", timeout.Milliseconds())
			os.Exit(0)
		})

		defer timeoutFunc.Stop()

		for _, entry := range ops {
			log.Printf("cleaning up: %s", entry.Name)

			// A failed step must not abort the rest: the later steps are the ones
			// that release leases and mark the agent offline.
			if err := entry.Op(ctx); err != nil {
				log.Printf("%s: clean up failed: %s", entry.Name, err.Error())
				continue
			}

			log.Printf("%s was shutdown gracefully", entry.Name)
		}

		close(wait)
	}()

	return wait
}
