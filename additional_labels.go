package prometheus_backfill

type AdditionalLabels interface {
	GetAdditionalLabels() map[string]string
}
