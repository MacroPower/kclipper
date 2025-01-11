package helmutil

import (
	"os"
	"sync"

	"github.com/MacroPower/kclipper/pkg/helm"
)

type ChartPkg struct {
	BasePath string
	Client   helm.ChartFileClient

	mu sync.RWMutex
}

func NewChartPkg(basePath string, client helm.ChartFileClient) *ChartPkg {
	return &ChartPkg{
		BasePath: basePath,
		Client:   client,
	}
}

func fileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return true
}
