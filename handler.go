package prometheus_backfill

import (
	"context"
	"fmt"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"go.uber.org/atomic"
	"golang.org/x/sync/semaphore"
	"math"
	"os"
	"runtime"
	"sync"
	"text/tabwriter"
	"time"
)

type backfillHandler struct {
	ch                  chan interface{}
	total               atomic.Int64
	done                atomic.Int64
	startTime           time.Time
	appender            storage.Appender
	blockWriter         *tsdb.BlockWriter
	writerLock          sync.Locker
	bstLock             sync.Locker
	ctx                 context.Context
	minTime             int64
	counter             int64
	bst                 *bst
	tmpWg               sync.WaitGroup // Temporary auxiliary waitGroup todo delete or give it a good usage
	maxParallelConsumes int64
	blockDuration       int64
	maxPerAppender      int64
	storeThreshold      int64
	outputDir           string
}

func NewPrometheusBackfillHandler(blockDuration, maxPerAppender, storeThreshold,
	maxParallelConsumes int64, ch chan interface{}, total int64, outputDir string) *backfillHandler {
	// make(chan interface{}, bufferedChanCap)
	bh := &backfillHandler{
		ch,
		atomic.Int64{},
		atomic.Int64{},
		time.Now(),
		nil,
		nil,
		&sync.Mutex{},
		&sync.Mutex{},
		context.Background(),
		math.MaxInt64, 0,
		new(bst),
		sync.WaitGroup{},
		maxParallelConsumes,
		blockDuration,
		maxPerAppender,
		storeThreshold,
		outputDir,
	}
	bh.total.Store(total)
	return bh
}

func (bh *backfillHandler) RunJob() {
	Notice("main", "Start parsing database")
	bh.done.Store(0)
	go bh.statusLoop()
	bh.listenOnChannel()
	bh.tmpWg.Wait()
	Notice("main", "End of parsing")
}

func (bh *backfillHandler) listenOnChannel() {
	Notice("Listening on channel")
	bh.setBlockWriter()
	sem := semaphore.NewWeighted(bh.maxParallelConsumes)
	var counter atomic.Int64
	for msg := range bh.ch {
		table := msg
		sem.Acquire(bh.ctx, 1)
		go func() {
			bh.checkAndStore(false)
			_ = counter.Inc()
			t := time.Now().UnixNano()
			bh.marshal(table)
			t = (time.Now().UnixNano() - t) / int64(time.Millisecond)
			// Notice("Marshaled", i, "(", reflect.TypeOf(table).String(), ") in", t, "ms. Status:", bh.done.Load(), "/", bh.total.Load())
			bh.done.Inc()
			sem.Release(1)
		}()
	}
	sem.Acquire(bh.ctx, bh.maxParallelConsumes)
	bh.checkAndStore(true)


	bh.writerLock.Lock()
	bh.flushBlockWriter()
	bh.writerLock.Unlock()
}

func (bh *backfillHandler) statusLoop() {
	w := tabwriter.NewWriter(os.Stdout, 1, 2, 5, ' ', tabwriter.DiscardEmptyColumns)
	mem := runtime.MemStats{}
	for {
		if bh.PrintStats(w, mem) {
			return
		}
		time.Sleep(time.Second * 10)
	}
}

func (bh *backfillHandler) PrintStats(w *tabwriter.Writer, mem runtime.MemStats) (endOfJob bool) {
	done := bh.done.Load()
	total := bh.total.Load()
	now := time.Now()
	fmt.Fprintln(w,"Progress\tProgress (%)\tRapidity\tStart time\tCurrent Time\tDuration")
	fmt.Fprintf(w, "%d/%d\t%.2f%%\t%.2f tables/s\t%s\t%s\t%s\n",
		done,
		total,
		float64(done)/float64(total)*100,
		float64(done)/now.Sub(bh.startTime).Seconds(),
		bh.startTime.Format(time.RFC3339),
		now.Format(time.RFC3339),
		now.Sub(bh.startTime))
	runtime.ReadMemStats(&mem)
	fmt.Fprintf(w, "mem.Alloc:\t%E\n", float64(mem.Alloc))
	fmt.Fprintf(w, "mem.TotalAlloc:\t%E\n", float64(mem.TotalAlloc))
	fmt.Fprintf(w, "mem.HeapAlloc:\t%E\n", float64(mem.HeapAlloc))
	fmt.Fprintf(w, "mem.NumGC:\t%E\n", float64(mem.NumGC))
	w.Flush()
	if done == total {
		return true
	}
	return false
}
