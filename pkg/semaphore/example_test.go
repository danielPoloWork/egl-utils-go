package semaphore_test

import (
	"context"
	"fmt"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/semaphore"
)

// Acquire bounds aggregate load rather than a raw operation count: a heavy job
// reserves more of the capacity than a light one.
func ExampleWeighted_Acquire() {
	sem := semaphore.NewWeighted(10)

	// Ten units admit one weight-6 job and one weight-4 job at the same time.
	if err := sem.Acquire(context.Background(), 6); err != nil {
		fmt.Println("acquire:", err)
		return
	}
	if err := sem.Acquire(context.Background(), 4); err != nil {
		fmt.Println("acquire:", err)
		return
	}
	fmt.Println("6 + 4 admitted")

	// The semaphore is full, so this third job waits for capacity to come back.
	// This example terminating is the proof that it did: the acquire below can
	// only complete after the Release that follows it, and nothing here depends
	// on how long that takes.
	admitted := make(chan struct{})
	go func() {
		if err := sem.Acquire(context.Background(), 6); err != nil {
			fmt.Println("acquire:", err)
			return
		}
		sem.Release(6)
		close(admitted)
	}()

	sem.Release(6)
	<-admitted
	sem.Release(4)

	fmt.Println("third job admitted after the release")
	// Output:
	// 6 + 4 admitted
	// third job admitted after the release
}
