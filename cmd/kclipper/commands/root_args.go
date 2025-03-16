package commands

type RootArgs struct {
	logLevel         *string
	logFormat        *string
	cpuProfile       *string
	memProfile       *string
	heapProfile      *string
	blockProfile     *string
	mutexProfile     *string
	memProfileRate   *int
	heapProfileRate  *int
	blockProfileRate *int
	mutexProfileRate *int
}

func NewRootArgs() *RootArgs {
	return &RootArgs{
		logLevel:         new(string),
		logFormat:        new(string),
		cpuProfile:       new(string),
		memProfile:       new(string),
		heapProfile:      new(string),
		blockProfile:     new(string),
		mutexProfile:     new(string),
		memProfileRate:   new(int),
		heapProfileRate:  new(int),
		blockProfileRate: new(int),
		mutexProfileRate: new(int),
	}
}

func (a *RootArgs) GetLogLevel() string {
	return *a.logLevel
}

func (a *RootArgs) GetLogFormat() string {
	return *a.logFormat
}

func (a *RootArgs) GetCPUProfile() string {
	return *a.cpuProfile
}

func (a *RootArgs) GetMemProfile() string {
	return *a.memProfile
}

func (a *RootArgs) GetHeapProfile() string {
	return *a.heapProfile
}

func (a *RootArgs) GetBlockProfile() string {
	return *a.blockProfile
}

func (a *RootArgs) GetMutexProfile() string {
	return *a.mutexProfile
}

func (a *RootArgs) GetMemProfileRate() int {
	return *a.memProfileRate
}

func (a *RootArgs) GetHeapProfileRate() int {
	return *a.heapProfileRate
}

func (a *RootArgs) GetBlockProfileRate() int {
	return *a.blockProfileRate
}

func (a *RootArgs) GetMutexProfileRate() int {
	return *a.mutexProfileRate
}
