package chaos

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Stressor handles CPU and Memory load injection within the Go runtime.
type Stressor struct{}

// NewStressor constructs a new Stressor instance.
func NewStressor() *Stressor {
	return &Stressor{}
}

// InjectCPU spawns busy-loop goroutines matching numCores for the specified duration.
// If numCores <= 0, it defaults to runtime.GOMAXPROCS(0).
func (s *Stressor) InjectCPU(ctx context.Context, duration time.Duration, numCores int) error {
	if numCores <= 0 {
		numCores = runtime.GOMAXPROCS(0)
	}

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(numCores)

	for i := 0; i < numCores; i++ {
		go func(coreID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Busy-wait loop to consume CPU cycles
					_ = 1 + 1
				}
			}
		}(i)
	}

	wg.Wait()
	return nil
}

// InjectMemory allocates a byte slice of target size in megabytes and retains it
// until the specified duration elapses or the context is canceled.
func (s *Stressor) InjectMemory(ctx context.Context, duration time.Duration, megabytes int) error {
	if megabytes <= 0 {
		return fmt.Errorf("memory allocation must be greater than 0 MB")
	}

	bytesToAllocate := int64(megabytes) * 1024 * 1024

	// Allocate heap memory and write bytes to ensure allocation isn't optimized away
	buffer := make([]byte, bytesToAllocate)
	for i := int64(0); i < bytesToAllocate; i += 4096 {
		buffer[i] = 1
	}

	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}

	// Keep buffer referenced until here to prevent premature GC sweep
	runtime.KeepAlive(buffer)
	return nil
}
