package prometheus_backfill

import (
	io_prometheus_client "github.com/prometheus/client_model/go"
	"reflect"
	"strings"
	"sync"
)

// [CONCUR] Rows are parsed concurrently
func (bh *backfillHandler) marshal(table interface{}) {
	list := reflect.ValueOf(table)
	if list.Kind() == reflect.Ptr {
		list = list.Elem()
	}
	if list.Kind() != reflect.Slice {
		// TODO Handle not list model
		panic("It's not a list. Implement me")
	}

	wg := sync.WaitGroup{}
	for i := 0; i < list.Len(); i++ { // Concurrent rows insertion
		structValue := reflect.ValueOf(list.Index(i).Interface())
		structType := reflect.TypeOf(list.Index(i).Interface())
		wg.Add(1)
		go func() {
			if structValue.Kind() == reflect.Ptr {
				structValue = structValue.Elem()
			}
			if structType.Kind() == reflect.Ptr {
				structType = structType.Elem()
			}
			GetAdditionalLabels := structValue.MethodByName("GetAdditionalLabels")
			timestamp := structValue.FieldByName("Timestamp").Int() * 1000 // See expfmt/openmetrics_create.go:348
			var row []*io_prometheus_client.Metric                         // same time stamp
			bh.deepReflectParse(timestamp, structType, structValue, GetAdditionalLabels, &row)
			bh.bstLock.Lock()
			bh.bst.insert(row)
			bh.bstLock.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
}

// deepReflectParse, finally, inserts a metric into the bst
func (bh *backfillHandler) deepReflectParse(timestamp int64, structType reflect.Type,
	structValue reflect.Value, GetAdditionalLabels reflect.Value, row *[]*io_prometheus_client.Metric) {
	for i := 0; i < structType.NumField(); i++ { // columns
		st := structType.Field(i)
		field := structValue.Field(i)
		switch field.Kind() {
		case reflect.Struct:
			bh.deepReflectParse(timestamp, reflect.TypeOf(field.Interface()), field, GetAdditionalLabels, row)
		case reflect.Float64:
			metric := bh.makeMetric(st, field.Float(), &timestamp, GetAdditionalLabels)
			if metric != nil {
				*row = append(*row, metric)
			}
		case reflect.Int64:
			metric := bh.makeMetric(st, float64(field.Int()), &timestamp, GetAdditionalLabels)
			if metric != nil {
				*row = append(*row, metric)
			}
		default:
		}
	}
}

func (bh *backfillHandler) makeMetric(st reflect.StructField, metricValue float64,
	ts *int64, GetAdditionalLabels reflect.Value) (metric *io_prometheus_client.Metric) {

	metricName := st.Name
	metricLabels := bh.getPrometheusLabels(st.Tag.Get("prometheus"))
	if metricLabels["metric_name"] != "" {
		//TODO need validation?
		metricName = metricLabels["metric_name"]
	}
	if len(metricLabels) == 0 {
		return nil
	}
	metric = new(io_prometheus_client.Metric)
	if !reflect.ValueOf(GetAdditionalLabels).IsZero() {
		mapp := GetAdditionalLabels.Call([]reflect.Value{})[0].MapRange()
		for mapp.Next() {
			k := mapp.Key()
			v := mapp.Value()
			metricLabels[k.String()] = v.String()
		}
	}

	metricType, ok := metricLabels["metric_type"]
	if !ok || metricType == "-" { // Ignore unwanted metrics
		return
	}
	var _ io_prometheus_client.MetricType // MetricType
	switch metricType {
	case "counter": // TODO
		_ = io_prometheus_client.MetricType_COUNTER
		if !strings.HasSuffix(metricName, "_total") {
			// expfmt.MetricFamiliyToOpenMetric row 91
			metricName += "_total"
		}
		metric.Counter = &io_prometheus_client.Counter{
			Value: &metricValue,
		}
	case "gauge":
		_ = io_prometheus_client.MetricType_GAUGE
		metric.Gauge = &io_prometheus_client.Gauge{
			Value: &metricValue,
		}
	case "histogram": // TODO
		_ = io_prometheus_client.MetricType_HISTOGRAM
		panic("implement me")
	case "summary": // TODO
		_ = io_prometheus_client.MetricType_SUMMARY
		panic("implement me")
	default: // TODO
		_ = io_prometheus_client.MetricType_UNTYPED
		panic("implement me")
	}
	var metricHelp = new(string)
	*metricHelp, ok = metricLabels["help"]
	if !ok {
		metricHelp = nil
	}
	delete(metricLabels, "metric_type")
	metricLabels["__name__"] = metricName
	metric.Label = bh.marshalLabelsMap(metricLabels)
	metric.TimestampMs = ts
	return
}

func (*backfillHandler) marshalLabelsMap(labels map[string]string) (prometheusLabels []*io_prometheus_client.LabelPair) {
	for k, v := range labels {
		name := k
		value := v
		prometheusLabels = append(prometheusLabels, &io_prometheus_client.LabelPair{
			Name:  &name,
			Value: &value,
		})
	}
	return
}

// E.g. prometheus:"metric_type:counter,unit:W,myLabel1:myLabel1Value..."
func (*backfillHandler) getPrometheusLabels(tag string) (labels map[string]string) {
	labels = make(map[string]string)
	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' { // || tag[i+1] != '"' {
			break
		}
		name := tag[:i]
		tag = tag[i+1:]

		// Scan quoted string to find value. // NOT QUOTED within the tag value
		i = 0 // = 1 (quoted)
		// '"' int the tag parsing outer version a tag value is represented within double-quotes
		// The inner value of the prometheus tag is expressed rather like the gorm tag
		// E.g. prometheus:"metric_type:counter,unit:W,myLabel1:myLabel1Value..."
		for i < len(tag) && tag[i] != ',' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		/*if i >= len(tag) { NOT NEEDED SINCE LAST CHARACTER IS NOT A '"'
			break
		}*/
		// i-th character could be a ',' or the last character of the tag value
		value := tag[:i]

		labels[name] = value
		// if i-th character is a ',' then we would continue scanning tag[i+1:]
		// 	if no labels are available after the ',', tag will be the empty string and the for loop condition will fail
		// if i == len(tag), tag[i+1:] would lead to an invalid slice index error
		if i != len(tag) {
			tag = tag[i+1:]
		}

	}
	return
}
