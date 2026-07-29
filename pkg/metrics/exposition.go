package metrics

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

// The two metric families, their help text and the exposition media type.
//
// contentType is the Prometheus text exposition format, version 0.0.4 — the
// universally understood baseline. It is emitted unconditionally: the Accept
// header is not negotiated, so there is no protobuf and no OpenMetrics variant
// (ADR-0050).
const (
	counterName = "http_requests_total"
	counterHelp = "Total number of HTTP requests, by method and response status code."

	durationName = "http_request_duration_seconds"
	durationHelp = "HTTP request latency in seconds, by method and response status code."

	contentType = "text/plain; version=0.0.4; charset=utf-8"
)

// defBuckets is the Prometheus default latency ladder, in seconds, reproduced
// verbatim from prometheus.DefBuckets — the buckets ADR-0027 chose and every
// dashboard written against v1 expects. Changing them would silently invalidate
// existing histogram_quantile queries, so they are frozen here rather than made
// configurable.
var defBuckets = [...]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// bucketLabels holds the rendered `le` label value for each bucket, plus the
// "+Inf" overflow at the end. Formatting them once at init keeps the exposition
// path free of per-bucket float conversion, and pins the spelling: 'g' with
// precision -1 is the shortest round-tripping form, which is what renders 1 as
// "1" and 0.005 as "0.005" — byte-identical to the reference encoder.
var bucketLabels = func() [len(defBuckets) + 1]string {
	var out [len(defBuckets) + 1]string
	for i, upper := range defBuckets {
		out[i] = strconv.FormatFloat(upper, 'g', -1, 64)
	}
	out[len(defBuckets)] = "+Inf"
	return out
}()

// seriesKey identifies one (method, code) time series. The status code is kept
// as an int on purpose: it is the map key, and an int costs no allocation where
// formatting the code to a string on every request would. The string form is
// materialised once, when the series is created.
type seriesKey struct {
	method string
	code   int
}

// series is the counter and histogram for one (method, code) pair.
//
// Every field is an atomic, so recording holds no lock at all: the registry's
// RWMutex is taken only to find (or create) the series. Guarding a series with
// its own mutex would have been simpler and would have given a scrape an exactly
// consistent snapshot, but in a real server most requests share one label pair —
// so that mutex would serialise the hot path on a single lock, which is the
// mistake ADR-0038 already paid to learn in cache.
//
// buckets counts observations *per* bucket, not cumulatively, so one observation
// is one atomic add rather than up to twelve; the cumulative form the exposition
// format requires is computed while rendering. The final slot is the overflow
// above the last bucket, reachable only through "+Inf".
type series struct {
	method string
	code   string

	buckets [len(defBuckets) + 1]atomic.Uint64
	sumBits atomic.Uint64 // float64 bits; see addFloat
	count   atomic.Uint64
}

// observe records one request of the given latency.
//
// The order of the three updates is deliberate: count is incremented last, so a
// concurrent scrape can only ever see a count that is *behind* the buckets and
// the sum, never ahead of them. The skew is therefore one-directional and
// bounded by the number of in-flight requests, and the invariant a consumer is
// most likely to lean on — the "+Inf" bucket is never smaller than the observed
// count — always holds.
func (s *series) observe(seconds float64) {
	s.buckets[bucketIndex(seconds)].Add(1)
	addFloat(&s.sumBits, seconds)
	s.count.Add(1)
}

// bucketIndex returns the slot for an observation: the first bucket whose upper
// bound it does not exceed, or the overflow slot. The bounds are inclusive
// (`le`, less-than-or-equal), which is what makes exactly 10s land in the last
// real bucket and 10.0001s in the overflow. A linear scan of eleven bounds beats
// a binary search here, and real latencies exit it early.
func bucketIndex(seconds float64) int {
	for i, upper := range defBuckets {
		if seconds <= upper {
			return i
		}
	}
	return len(defBuckets)
}

// addFloat atomically adds v to a float64 stored as its bit pattern. Go has no
// atomic float, so the value is carried in a Uint64 and updated by
// compare-and-swap; the loop retries only when another observation lands in
// between.
func addFloat(dst *atomic.Uint64, v float64) {
	for {
		old := dst.Load()
		updated := math.Float64bits(math.Float64frombits(old) + v)
		if dst.CompareAndSwap(old, updated) {
			return
		}
	}
}

// snapshot is one series read out for exposition: plain numbers, no atomics, so
// the rendering code cannot accidentally re-read a value that has since moved.
type snapshot struct {
	method  string
	code    string
	buckets [len(defBuckets) + 1]uint64 // cumulative, as the format requires
	sum     float64
	count   uint64
}

// collect reads every series into a slice sorted the way the exposition format
// wants it: by label value, in the order the labels are emitted (code first,
// then method, because label names are sorted alphabetically).
//
// The registry lock is held only for the map walk. The atomics are read after it
// is released, which is why a scrape sees a point-in-time-ish rather than an
// exactly consistent view — see series.
func (r *Recorder) collect() []snapshot {
	r.mu.RLock()
	live := make([]*series, 0, len(r.series))
	for _, s := range r.series {
		live = append(live, s)
	}
	r.mu.RUnlock()

	out := make([]snapshot, 0, len(live))
	for _, s := range live {
		snap := snapshot{
			method: s.method,
			code:   s.code,
			sum:    math.Float64frombits(s.sumBits.Load()),
			count:  s.count.Load(),
		}
		// Cumulate on the way out: bucket i of the exposition is every
		// observation at or below its bound.
		running := uint64(0)
		for i := range s.buckets {
			running += s.buckets[i].Load()
			snap.buckets[i] = running
		}
		out = append(out, snap)
	}

	// slices.SortFunc rather than sort.Slice: the reflection-based swapper the
	// latter builds allocates on every scrape, and this comparison is trivially
	// expressible generically.
	slices.SortFunc(out, func(a, b snapshot) int {
		if c := strings.Compare(a.code, b.code); c != 0 {
			return c
		}
		return strings.Compare(a.method, b.method)
	})
	return out
}

// appendExposition renders every series into buf in Prometheus text exposition
// format and returns the extended buffer.
//
// Two properties are load-bearing and both come from the reference encoder,
// which this was verified against byte for byte (ADR-0050,
// testdata/exposition.golden):
//
//   - Families are emitted in sorted name order, so the histogram precedes the
//     counter ("http_request_d…" < "http_requests_…").
//   - A family with no series is omitted entirely — HELP and TYPE included — so
//     a Recorder that has seen no traffic exposes an empty body, which is a
//     valid scrape rather than a set of zero-valued lies.
func (r *Recorder) appendExposition(buf []byte) []byte {
	snaps := r.collect()
	if len(snaps) == 0 {
		return buf
	}

	// Size the buffer once from the series count. Each series renders as one line
	// per bucket plus _sum, _count and the counter, at roughly eighty bytes a
	// line; growing by doubling instead costs an allocation ladder on every
	// scrape, which measurement put at nine allocations where three will do.
	const perSeries = (len(defBuckets) + 4) * 80
	if need := len(snaps)*perSeries + 256; cap(buf)-len(buf) < need {
		grown := make([]byte, len(buf), len(buf)+need)
		copy(grown, buf)
		buf = grown
	}

	buf = append(buf, "# HELP "+durationName+" "+durationHelp+"\n"...)
	buf = append(buf, "# TYPE "+durationName+" histogram\n"...)
	for _, s := range snaps {
		for i, cumulative := range s.buckets {
			buf = append(buf, durationName+"_bucket{code=\""...)
			buf = append(buf, s.code...)
			buf = append(buf, "\",method=\""...)
			buf = append(buf, s.method...)
			buf = append(buf, "\",le=\""...)
			buf = append(buf, bucketLabels[i]...)
			buf = append(buf, "\"} "...)
			buf = strconv.AppendUint(buf, cumulative, 10)
			buf = append(buf, '\n')
		}
		buf = append(buf, durationName+"_sum"...)
		buf = r.appendLabels(buf, s)
		buf = strconv.AppendFloat(buf, s.sum, 'g', -1, 64)
		buf = append(buf, '\n')

		buf = append(buf, durationName+"_count"...)
		buf = r.appendLabels(buf, s)
		buf = strconv.AppendUint(buf, s.count, 10)
		buf = append(buf, '\n')
	}

	buf = append(buf, "# HELP "+counterName+" "+counterHelp+"\n"...)
	buf = append(buf, "# TYPE "+counterName+" counter\n"...)
	for _, s := range snaps {
		buf = append(buf, counterName...)
		buf = r.appendLabels(buf, s)
		// The request counter and the histogram's _count are the same number by
		// construction now that one atomic backs both, where two independent
		// collectors could drift apart between updates.
		buf = strconv.AppendUint(buf, s.count, 10)
		buf = append(buf, '\n')
	}
	return buf
}

// appendLabels writes `{code="…",method="…"} ` — the label set without `le`,
// alphabetically ordered, trailing space included.
func (*Recorder) appendLabels(buf []byte, s snapshot) []byte {
	buf = append(buf, "{code=\""...)
	buf = append(buf, s.code...)
	buf = append(buf, "\",method=\""...)
	buf = append(buf, s.method...)
	buf = append(buf, "\"} "...)
	return buf
}
