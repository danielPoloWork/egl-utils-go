package metrics

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// goldenObservations is the exact input that produced testdata/exposition.golden
// under prometheus/client_golang + prometheus/common/expfmt, captured while that
// SDK was still a dependency (ADR-0050). It spreads observations across the first
// bucket, several middles, exactly the top bound, and past it into the "+Inf"
// overflow, over three label pairs including the normalized "other" method.
var goldenObservations = []struct {
	method string
	code   int
	secs   []float64
}{
	{"GET", 200, []float64{0.001, 0.003, 0.02, 0.2, 1.5, 42}},
	{"POST", 404, []float64{0.007}},
	{"other", 418, []float64{9.999, 10, 10.0001}},
}

// TestExpositionMatchesTheReferenceEncoder is the conformance test for the whole
// point of ADR-0050: that hand-written exposition text is byte-for-byte what the
// Prometheus client library would have emitted.
//
// The golden file is not a record of what this code does — it is the reference
// encoder's output, produced by the real SDK before it was removed, and committed
// so the check survives the dependency. If this fails, the writer has drifted
// from the format, not from a previous version of itself.
func TestExpositionMatchesTheReferenceEncoder(t *testing.T) {
	want, err := os.ReadFile("testdata/exposition.golden")
	require.NoError(t, err)

	r := New()
	for _, o := range goldenObservations {
		for _, s := range o.secs {
			r.observe(o.method, o.code, s)
		}
	}

	got := r.appendExposition(nil)
	require.Equal(t, string(want), string(got),
		"exposition must match prometheus/common/expfmt byte for byte")
}

// TestEmptyRecorderExposesNothing pins the omission rule: the reference encoder
// emits no family — not even HELP and TYPE — for a family with no series, so a
// Recorder that has seen no traffic must produce an empty body rather than a set
// of zero-valued lines that would read as real observations.
func TestEmptyRecorderExposesNothing(t *testing.T) {
	require.Empty(t, New().appendExposition(nil))
}

// TestBucketBoundsAreInclusive pins `le` semantics: an observation exactly equal
// to a bound belongs to that bucket, not the next one. Off by one here would skew
// every histogram_quantile a consumer computes.
func TestBucketBoundsAreInclusive(t *testing.T) {
	for i, upper := range defBuckets {
		require.Equal(t, i, bucketIndex(upper), "%v must land in bucket %d", upper, i)
	}
	require.Equal(t, 0, bucketIndex(0), "a zero-second request lands in the first bucket")
	require.Equal(t, len(defBuckets), bucketIndex(defBuckets[len(defBuckets)-1]+0.0001),
		"past the last bound is the overflow slot")
}

// TestCumulativeBucketsAndInfEqualsCount pins the format's cumulative rule and
// the invariant a consumer is most likely to lean on.
func TestCumulativeBucketsAndInfEqualsCount(t *testing.T) {
	r := New()
	for _, s := range []float64{0.001, 0.001, 0.3, 99} {
		r.observe("GET", 200, s)
	}
	snaps := r.collect()
	require.Len(t, snaps, 1)

	got := snaps[0].buckets
	for i := 1; i < len(got); i++ {
		require.GreaterOrEqual(t, got[i], got[i-1], "bucket %d must not decrease", i)
	}
	require.Equal(t, uint64(2), got[0], "two observations at or below 5ms")
	require.Equal(t, snaps[0].count, got[len(got)-1], `the "+Inf" bucket equals the count`)
	require.InDelta(t, 99.302, snaps[0].sum, 1e-9)
}

// TestSeriesAreSortedByLabelValue pins the emission order the reference encoder
// uses: label names sorted alphabetically, so series sort by code and then by
// method. A scrape whose series order wandered would still parse, but the golden
// comparison is only meaningful if the order is deterministic.
func TestSeriesAreSortedByLabelValue(t *testing.T) {
	r := New()
	for _, o := range []struct {
		method string
		code   int
	}{{"POST", 500}, {"GET", 200}, {"other", 500}, {"GET", 404}} {
		r.observe(o.method, o.code, 0.01)
	}

	var order []string
	for _, s := range r.collect() {
		order = append(order, s.code+"/"+s.method)
	}
	require.Equal(t, []string{"200/GET", "404/GET", "500/POST", "500/other"}, order)
}

// TestAddFloatAccumulates covers the compare-and-swap float accumulator directly,
// including the case Go has no atomic for: adding to a value another goroutine
// may have just changed.
func TestAddFloatAccumulates(t *testing.T) {
	r := New()
	r.observe("GET", 200, 0.5)
	r.observe("GET", 200, 0.25)
	require.InDelta(t, 0.75, r.collect()[0].sum, 1e-12)
}

// TestConcurrentObserveLosesNothing is the -race target for the lock-free
// recording path: many goroutines sharing one label pair, which is the shape a
// real server produces and the reason a per-series mutex was rejected.
func TestConcurrentObserveLosesNothing(t *testing.T) {
	const goroutines, each = 16, 500

	r := New()
	done := make(chan struct{})
	for g := range goroutines {
		go func(g int) {
			defer func() { done <- struct{}{} }()
			for range each {
				// Two label pairs, so series creation races too.
				r.observe("GET", 200+g%2, 0.02)
			}
		}(g)
	}
	for range goroutines {
		<-done
	}

	var total uint64
	for _, s := range r.collect() {
		total += s.count
	}
	require.Equal(t, uint64(goroutines*each), total, "every observation must be counted exactly once")
}

// TestLosingTheCreationRaceKeepsTheLiveSeries covers newSeries' double check
// under the write lock, and pins why it exists: when two goroutines both find no
// series and both go on to create one, the loser must adopt the winner's series
// rather than install a second. Overwriting it would silently discard whatever
// the winner had already counted.
//
// Calling newSeries twice reproduces exactly the state the loser arrives in, so
// the branch is pinned deterministically instead of being left to a race the
// scheduler may or may not produce.
func TestLosingTheCreationRaceKeepsTheLiveSeries(t *testing.T) {
	r := New()
	key := seriesKey{method: "GET", code: 200}

	winner := r.newSeries(key)
	winner.observe(0.01)
	loser := r.newSeries(key)

	require.Same(t, winner, loser, "the second creator must adopt the live series")
	require.Len(t, r.series, 1)
	require.Equal(t, uint64(1), r.collect()[0].count, "the winner's observation must survive")
}

// TestSeriesCountIsBoundedByConstruction demonstrates the cardinality claim in
// the package doc: junk methods all collapse onto "other", so the series count
// is bounded by the label domains rather than by what a client sends.
func TestSeriesCountIsBoundedByConstruction(t *testing.T) {
	r := New()
	for i := range 1000 {
		r.observe(normalizeMethod("JUNK"+strconv.Itoa(i)), 200, 0.01)
	}
	snaps := r.collect()
	require.Len(t, snaps, 1, "a thousand distinct method tokens must make one series")
	require.Equal(t, "other", snaps[0].method)
	require.Equal(t, uint64(1000), snaps[0].count)
}

// TestBucketLabelsSpelling pins the float formatting that makes the golden match:
// the shortest round-tripping form, so 1 renders as "1" and not "1.0".
func TestBucketLabelsSpelling(t *testing.T) {
	require.Equal(t,
		[]string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10", "+Inf"},
		strings.Split(strings.Join(bucketLabels[:], " "), " "))
}
