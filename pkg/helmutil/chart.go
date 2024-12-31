package helmutil

import (
	"os"
	"sync"

	"github.com/MacroPower/kclx/pkg/helm"
)

type ChartPkg struct {
	BasePath string
	Client   helm.ChartClient

	mu sync.RWMutex
}

func NewChartPkg(basePath string, client helm.ChartClient) *ChartPkg {
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
