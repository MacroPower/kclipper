package commands

type RootArgs struct {
	logLevel  *string
	logFormat *string
}

func NewRootArgs() *RootArgs {
	return &RootArgs{
		logLevel:  new(string),
		logFormat: new(string),
	}
}

func (a *RootArgs) GetLogLevel() string {
	return *a.logLevel
}

func (a *RootArgs) GetLogFormat() string {
	return *a.logFormat
}
