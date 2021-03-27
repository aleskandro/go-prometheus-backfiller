# Alibaba dataset bulk-import Example

This example will use the sample .parquet files in the [input/](input/) folder to bulk-import data from the Alibaba Cluster Trace
as TSDB blocks for Prometheus.

To use it:
```bash
cd /path/to/this/repo/examples/alibaba
go run main.go
```

Then deploy prometheus using the docker-compose available in the /path/to/this/repo/prometeus-deploy folder.
