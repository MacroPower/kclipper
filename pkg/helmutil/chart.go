package helmutil

import "sync"

type ChartPkg struct {
	BasePath string

	mu sync.RWMutex
}

func NewChartPkg(basePath string) *ChartPkg {
	return &ChartPkg{
		BasePath: basePath,
	}
}
