package helmutil

import (
	"os"
	"sync"
)

type ChartPkg struct {
	BasePath string

	mu sync.RWMutex
}

func NewChartPkg(basePath string) *ChartPkg {
	return &ChartPkg{
		BasePath: basePath,
	}
}

func fileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return true
}
