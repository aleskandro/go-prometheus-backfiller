package prometheus_backfill

import (
	"github.com/go-kit/kit/log"
	io_prometheus_client "github.com/prometheus/client_model/go"
	labels2 "github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/tsdb"
	"math"
	"sort"
	"time"
)

type auxStoreStruct struct {
	metric *io_prometheus_client.Metric
	labels []labels2.Label
}

// [CONCUR] Launch the store in tsdb as a go routine
func (bh *backfillHandler) checkAndStore(force bool) {
	bh.bstLock.Lock()
	if !force && bh.bst.length < bh.storeThreshold {
		bh.bstLock.Unlock()
		return
	}
	oldBst := bh.bst
	bh.bst = new(bst)
	bh.bstLock.Unlock()
	Notice2("BST swap done: ", oldBst.length, "metrics to store. Store in appender...")
	counter := 0
	var toStore []auxStoreStruct
	oldBst.inorder(func(metric []*io_prometheus_client.Metric) {
		// Each metric array reports the same timestamp with different labels,
		// Need group by name
		for _, m := range metric {
			counter++

			var labels labels2.Labels
			for _, l := range m.Label {
				labels = append(labels, labels2.Label{
					Name:  *l.Name,
					Value: *l.Value,
				})
			}
			sort.Slice(labels, func(i int, j int) bool {
				return labels[i].Name < labels[j].Name
			})
			toStore = append(toStore, auxStoreStruct{
				metric: m,
				labels: labels,
			})
		}
	})

	bh.writerLock.Lock()
	for _, m := range toStore {
		bh.store(&m)
	}
	bh.writerLock.Unlock()
	Notice3("Saved", counter, "saved into tsdb appender of which length is:", bh.counter)
}

func (bh *backfillHandler) store(m *auxStoreStruct) {
	switch {
	case m.metric.Gauge != nil:
		bh.toTsdb(m.metric, m.metric.Gauge.Value, m.labels)
	default: // TODO
		panic("implement me")
	}
}

func (bh *backfillHandler) toTsdb(m *io_prometheus_client.Metric, v *float64, labels []labels2.Label) {

	// bh.writerLock.Lock()
	if bh.minTime == math.MaxInt64 {
		// Putting first metric, the appender will set its lower bound timestamp
		bh.minTime = *m.TimestampMs
	}
	if bh.minTime != math.MaxInt64 && !(bh.minTime <= *m.TimestampMs && *m.TimestampMs <= (bh.minTime+(bh.blockDuration*1000))) {
		// current time stamp is not in the appender allowed range. Creating a new block
		Notice3("Not within the allowed time-interval range",
			time.Unix(bh.minTime/1000, 0), "<=",
			time.Unix(*m.TimestampMs/1000, 0).String(), "<=",
			time.Unix((bh.minTime+bh.blockDuration)/1000, 0).String(), "blockDuration (ms): ",
			bh.blockDuration)
		bh.setBlockWriter()
		bh.minTime = *m.TimestampMs
	}
	if _, err := bh.appender.Add(labels, *m.TimestampMs, *v); err != nil {
		ErrLog("Error appending metric", m.String()[0:100], err, '\n')
	}
	bh.counter++
	if bh.counter >= bh.maxPerAppender {
		Notice("Flushing appender (writes on Disk)...")
		bh.setBlockWriter()
		bh.counter = 0
		bh.minTime = math.MaxInt64
	}
}

// setBlockWriter is not thread-safe! Use with the writerLock
func (bh *backfillHandler) setBlockWriter() {
	if bh.blockWriter != nil && bh.appender != nil {
		bh.flushBlockWriter()
	}
	bh.blockWriter, _ = tsdb.NewBlockWriter(log.NewNopLogger(), bh.outputDir, bh.blockDuration)
	bh.appender = bh.blockWriter.Appender(bh.ctx)
}

// flushBlockWriter is not thread-safe! Use with the writerLock
func (bh *backfillHandler) flushBlockWriter() {
	if err := bh.appender.Commit(); err != nil {
		ErrLog("Error on appender commit: ", bh.appender, bh.blockWriter)
	}
	ulid, err := bh.blockWriter.Flush(bh.ctx)
	if err != nil {
		ErrLog("Error flushing block", ulid, err)
	}
	if err := bh.blockWriter.Close(); err != nil {
		ErrLog("Error closing block", ulid, err)
	}
	Notice3("Block written, flushed (new appender)", ulid.String())
}
