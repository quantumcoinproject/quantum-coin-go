package influxdb

import (
	uurl "net/url"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/metrics"
)

// TestUBF127_ReporterStopsTickers checks that the reporter's tickers are stoppable and
// that run tears them down when it returns. Upstream 297ec0669: run used time.Tick,
// whose underlying runtime Ticker can never be stopped or garbage collected.
func TestUBF127_ReporterStopsTickers(t *testing.T) {
	// Point at a closed port: send() will fail and log, which is all we need. The
	// client is created lazily and does not dial here.
	u, err := uurl.Parse("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	r := &reporter{
		reg:      metrics.NewRegistry(),
		interval: 20 * time.Millisecond,
		url:      *u,
		cache:    make(map[string]int64),
		stop:     make(chan struct{}),
	}
	if err := r.makeClient(); err != nil {
		t.Fatalf("cannot make client: %v", err)
	}

	done := make(chan struct{})
	go func() {
		r.run()
		close(done)
	}()

	// Let the interval ticker fire a few times, which also verifies the loop still
	// selects on the right channel after the time.Tick -> time.NewTicker switch.
	time.Sleep(100 * time.Millisecond)
	close(r.stop)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reporter.run did not return, tickers cannot be stopped")
	}
}
