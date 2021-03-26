# Prometheus Backfill Golang library

### TODO

# Resources

### TODO

## Considerations

### TODO 

# Example usage

Define data models and controllers to get your historical data:

```go
package models

type BaseRecord struct {
    // Name of the field will be used as metricName
    // prometheus tag needs metric_type:gauge (other metrics are not supported yet)
    // Any other field in the prometheus tag will be used as label for the metric.
	SystemRunning         float64 `gorm:"column:system" prometheus:"metric_type:gauge,description:The system is up and running,humanName: "System is running"`
	ManagementRunning     float64 `gorm:"column:management" prometheus:"metric_type:gauge"`
	TotalNodesRunning     float64 `gorm:"column:nodes_total" prometheus:"metric_type:gauge"`
}

// If this method is present, it will be used to attach other labels to each metric
func (br BaseRecord) GetAdditionalLabels() map[string]string {
	return map[string]string{
		"MyLabel1":  "MyValue1",
		"MyLabel2":  func() string {
                        return "my value"
                     }(),
	}
}

```

```go
package main

var (
	blockDuration     = int64(24 * 15 * time.Hour / time.Millisecond) // Duration of the prometheus block. 
	        // It depends on your data... It will also be processed by Prometheus compaction when you will start
            // your prometheus instance with these data
	maxPerAppender    = int64(100e6) // 100M metrics (rows * columns) => this will limit the amount of used ram
	storeBstThreshold = int64(1e3) // 1k rows (visiting the tree is expensive, so keep it small)
	bufferedChanCap   = 128 // The following values depend both on available CPUs and Memory
	semaphoreWeight   = int64(32) // This is capped by synchronization structures (marshalling, write locks and writes on appender and disk)
	concurrentQueries = int64(32)
)

func LaunchPrometheusBackfill() {
    totalNumberOfMessagesWillBeSent := 2e4 // The number of total tables to send
	ch := make(chan interface{}, bufferedChanCap)
	bh := prometheus_backfill.NewPrometheusBackfillHandler(blockDuration, maxPerAppender, 
        storeBstThreshold, semaphoreWeight, ch,
	    totalNumberOfMessagesWillBeSent, "/tmp/tsdb",
    )
	go parseData(ch)
	bh.RunJob() // This method will consume messages sent to the channel and convert them into tsdb

	// Printing stats at the end of the job
	w := tabwriter.NewWriter(os.Stdout, 1, 2, 5, ' ', tabwriter.DiscardEmptyColumns)
	mem := runtime.MemStats{}
	bh.PrintStats(w, mem)
}

func parseData(ch chan interface{}) {
	// YOUR QUERIES
	for _, table := range tableModels {
		ch <- table
	}
	close(ch)
}
```

### NOTES

- table has to be a list of object instances defined as the struct `BaseRecord` above;
- By now, the struct has to contain a Timestamp field in *seconds*.

A complete example of the code to adapt data to TSDB is available in the examples/alibaba directory. 
There, the [Alibaba cluster trace](https://github.com/alibaba/clusterdata) is parsed from pre-processed
 `.parquet` files and stored as TSDB. 
 
The values used to tune the job are based on a Core i7-5760K with 40GB RAM. The job uses at most 10GB of RAM to complete
successfully.
 
# Limitations and TODOs

- Only supports Gauge: counters should be straight forward, histograms/summary adapters have to be thought
- Auto-tuning of parameters based on available resources

# Importing into prometheus

An example of a dockerFile and prometheus configuration to import data parsed from the go-prometheus-backfiller is 
available at 