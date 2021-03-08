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
