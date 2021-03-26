package prometheus_backfill

import (
	"github.com/fatih/color"
)

var (
	ErrLog  = color.New(color.FgRed).PrintfFunc()
	Notice  = color.New(color.FgGreen).PrintlnFunc()
	Notice2 = color.New(color.FgHiBlue).PrintlnFunc()
	Notice3 = color.New(color.FgCyan, color.Bold).PrintlnFunc()
	Notice4 = color.New(color.FgYellow).PrintlnFunc()
)

func Must(err error, errString string) {
	if err != nil {
		ErrLog("%v\n%v\n", err, errString)
		panic(err)
	}
}

func Should(err error, errString string) bool {
	if err != nil {
		ErrLog("%v\n%v\n", err, errString)
		return false
	}
	return true
}
