package main

import (
	"alibaba_example/models"
	"context"
	"fmt"
	"github.com/aleskandro/go-prometheus-backfiller"
	"github.com/xitongsys/parquet-go-source/local"
	_ "github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/reader"
	"go.uber.org/atomic"
	"golang.org/x/sync/semaphore"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

var (
	blockDuration = int64(24 * 8 * time.Hour / time.Millisecond) // Duration of the prometheus block.
	// It depends on your data... It will also be processed by Prometheus compaction when you will start
	// your prometheus instance with these data
	maxPerAppender    = int64(100e6) // 100M metrics (rows * columns) => this will limit the amount of used ram
	storeBstThreshold = int64(1e3)   // 1k rows (visiting the tree is expensive, so keep it small)
	bufferedChanCap   = 64           // The following values depend both on available CPUs and Memory
	semaphoreWeight   = int64(32)    // This is capped by synchronization structures (marshalling, write locks and writes on appender and disk)
	concurrentQueries = int64(40)
	path              = "/run/media/aleskandro/9843e2e5-21ea-43cb-a51a-11ae0b323343/alibaba/exploded"
	outputDir         = "/run/media/aleskandro/9843e2e5-21ea-43cb-a51a-11ae0b323343/alibaba/prometheus"
)

func main() {
	files := make([]string, 0)
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		if strings.Contains(path, ".parquet") {
			files = append(files, path)
		}
		return nil
	})

	prometheus_backfill.Must(err, "unable to read file lists")

	prometheus_backfill.Notice2("reading number of chunks")
	num := new(atomic.Int64)
	num.Store(0)
	wg := sync.WaitGroup{}
	prometheus_backfill.Notice3("Number of files: ", len(files))
	for _, filePath := range files {
		wg.Add(1)
		filePath := filePath
		go func() {
			fr, err := local.NewLocalFileReader(filePath)
			prometheus_backfill.Must(err, "unable to open file")
			pr, err := reader.NewParquetReader(fr, new(models.ContainerUsage), 1) // TODO np number parallel
			prometheus_backfill.Must(err, "can't create a parquet reader")
			num.Add(int64(len(pr.Footer.RowGroups)))
			pr.ReadStop()
			prometheus_backfill.Must(fr.Close(), fmt.Sprintf("Unable to close file %s", filePath))
			wg.Done()
		}()
	}
	wg.Wait()
	LaunchPrometheusBackfill(files, num.Load())
}

func LaunchPrometheusBackfill(files []string, total int64) {
	ch := make(chan interface{}, bufferedChanCap)
	bh := prometheus_backfill.NewPrometheusBackfillHandler(blockDuration, maxPerAppender,
		storeBstThreshold, semaphoreWeight, ch,
		total, outputDir,
	)
	go parseData(files, ch)
	bh.RunJob() // This method will consume messages sent to the channel and convert them into tsdb

	// Printing stats at the end of the job
	w := tabwriter.NewWriter(os.Stdout, 1, 2, 5, ' ', tabwriter.DiscardEmptyColumns)
	mem := runtime.MemStats{}
	bh.PrintStats(w, mem)
}

func parseData(files []string, ch chan interface{}) {
	sem := semaphore.NewWeighted(concurrentQueries)
	for _, filePath := range files {
		_ = sem.Acquire(context.TODO(), 1)
		filePath := filePath
		go func() {
			fr, err := local.NewLocalFileReader(filePath)
			prometheus_backfill.Must(err, "unable to open file")
			pr, err := reader.NewParquetReader(fr, new(models.ContainerUsage), 1) // TODO np number parallel
			prometheus_backfill.Must(err, "can't create a parquet reader")
			rowGroups := pr.Footer.RowGroups
			num := int64(0)
			for _, rg := range rowGroups {
				chunk := make([]*models.ContainerUsage, rg.NumRows)
				err = pr.Read(&chunk)
				if err != nil {
					prometheus_backfill.ErrLog("row group reading error", err)
					sem.Release(1)
					return
				}
				num += rg.NumRows
				for _, row := range chunk {
					row.Timestamp += 1583020800 // set 2020-03-01 12.00.00 AM UTC as starting date
				}
				ch <- chunk

			}
			prometheus_backfill.Notice("Number of rows read: ", num, "(", filePath, ")")
			pr.ReadStop()
			prometheus_backfill.Must(fr.Close(), fmt.Sprintf("Unable to close file %s", filePath))
			sem.Release(1)
		}()
	}
	_ = sem.Acquire(context.TODO(), concurrentQueries)
	close(ch)
}
