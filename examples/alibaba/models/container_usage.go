package models

import "fmt"

type ContainerUsage struct {
	Id         string  `parquet:"name=id, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	Timestamp  int64   `parquet:"name=timestamp, type=INT64, repetitiontype=OPTIONAL"`
	Cpu        float64 `parquet:"name=cpu, type=DOUBLE, repetitiontype=OPTIONAL" `
	Mem        int64   `parquet:"name=mem, type=INT64, repetitiontype=OPTIONAL" prometheus:"metric_type:gauge"`
	NetIn      float64 `parquet:"name=net_in, type=DOUBLE, repetitiontype=OPTIONAL" `
	NetOut     float64 `parquet:"name=net_out, type=DOUBLE, repetitiontype=OPTIONAL" `
	Disk       float64 `parquet:"name=disk, type=DOUBLE, repetitiontype=OPTIONAL" `
	AppGroupId int64   `parquet:"name=aid, type=INT64, repetitiontype=OPTIONAL"`
}

// If this method is present, it will be used to attach other labels to each metric
func (br ContainerUsage) GetAdditionalLabels() map[string]string {
	return map[string]string{
		"ID":         br.Id,
		"AppGroupID": fmt.Sprintf("%d", br.AppGroupId),
	}
}
