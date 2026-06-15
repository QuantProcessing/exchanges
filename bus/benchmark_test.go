package bus

import (
	"context"
	"sync"
	"testing"
)

func BenchmarkBusPublishFanout(b *testing.B) {
	bus := New()
	drainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		sub := bus.Subscribe("execution", 64)
		defer sub.Close()
		wg.Add(1)
		go func(ch <-chan Envelope) {
			defer wg.Done()
			for {
				select {
				case <-drainCtx.Done():
					return
				case <-ch:
				}
			}
		}(sub.C())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(context.Background(), "execution", i); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	cancel()
	wg.Wait()
}
