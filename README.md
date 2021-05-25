# Prometheus Backfill Golang library

At current time, Prometheus doesn't allow any efficient strategy to import historical data from other monitoring systems
into a new installation of Prometheus (i.e., when migrating from a legacy monitoring system to Prometheus).

Prometheus team is working at this feature by implementing backfilling from OpenMetric raw data through promtool: proposal for importing from other formats are being discussed on different [issues](https://github.com/prometheus/prometheus/issues/7119).

This WIP back-filling library allows to define your own connectors to an old monitoring system (e.g., based on MySql, Parquet/Csv files, HTTP APIs...) and the models representing the metrics to convert into TSDB format. Then by using data structures of Prometheus, it will adapt by Go tags-based reflection the metrics from the old system and save them into a folder as TSDB blocks that Prometheus can easily read.

# Resources

As you will see below, implementing the bulk data import for prometheus with this go backfilling library consist of the following steps:
1. Writing the data models related to the metrics by using the `prometheus` tag to define metric type (at current time, only gauge and counters are supported) and static labels;
2. Writing a `func (br BaseRecord) GetAdditionalLabels() map[string]string` to fill other labels for specific metrics of a model instance (i.e. a set of metrics related to the same timestamp)
3. Writing a function that queries data from the old system (whatever it is) as a list of model instances (e.g., a list of rows from a Database) and send each list/chunk of data to a channel. This function must run as a go routine.
4. Finally, instantiating the PrometheusBackffillHandler by passing the instance of the channel on which the data are going to be sent and setting a few parameters to tune performance and usage of resources.

Parameters that have to be set are:

- `blockDuration` \[= int64(24 * 15 * time.Hour / time.Millisecond)\]: the duration of a [Prometheus Block](https://prometheus.io/docs/prometheus/latest/storage/#on-disk-layout). The greater the block the less fragmentation of data (and instances to instantiate to complete the job). Consider that Prometheus will do compaction of the blocks when you will deploy it with the back-filled data.

- `maxPerAppender` \[= int64(100e6)\] Maximum number of metrics to store in the memory appender. When this threshold is exceeeded the job will start flushing metrics to the disk. It will limit the amount of RAM used by the job.
- `storeBstThreshold` \[= int64(1e3)\] Before appending data to the [prometheus appender](https://github.com/prometheus/prometheus/blob/b3feb2c2aed8cd27f69613985ad7ddcab2cb1e6c/storage/interface.go#L159), data have to be sorted by time. A binary search tree is used as first arrival of group of metrics with the same time. Then traversal of the tree will provide data to the appender after the size of the BST reaches this threshold. Keep it lower than the maxPerAppender paremeter and don't set it too high because making the code thread-safe need blocking any access to the BST when performing the Swap to the appender. This would block all the threads for a lot of time if the BST is too big.
- `bufferedChanCap` \[= 128\] the maximum size of the channel used to provide data to the job. It will induce synchronization capabilities to the code and will limit the amount of data waiting to be processed in-memory.
- `semaphoreWeight` the maximum number of data (lists of model instances/lists sent to the channel) that can be concurrently consumed by the marshalling jobs
- \[Optional] `conccurrentQueries` you can define your queries go routine by limiting the maximum number of concurrent queries to perform. This depends on your code. Have a look at main.go in [Alibaba example](examples/alibaba/).

# Importing data to Prometheus

An example of a dockerFile and prometheus configuration to import data parsed from the go-prometheus-backfiller is 
available into the [prometheus-deploy folder](prometheus-deploy)

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

# Other resources

[Preprint@WetICE2021: Prometheus and AIOps for the orchestration ofCloud-native applications in Ananke](https://nc.alessandrodistefano.eu/s/gPd5MZyXjjkpjz4)
