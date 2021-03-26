module alibaba_example

go 1.15

// TODO comment me to use the latest official version of the backfiller
replace github.com/aleskandro/go-prometheus-backfiller v0.0.0 => ../../

require (
	github.com/aleskandro/go-prometheus-backfiller v0.0.0
	github.com/xitongsys/parquet-go v1.6.0
	github.com/xitongsys/parquet-go-source v0.0.0-20201108113611-f372b7d813be
)
