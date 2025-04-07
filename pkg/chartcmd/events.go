package chartcmd

type (
	// Sent when initialization has completed.
	EventInit struct {
		Err error
	}

	// Sent when an item has been added.
	EventAdded struct {
		Err error
	}

	// Sent to update the total chart count.
	EventSetChartTotal int

	// Sent when a chart update has started.
	EventUpdatingChart string

	// Sent when a chart has been updated, or when a fatal error occurs during an
	// update.
	EventUpdatedChart struct {
		Err   error
		Chart string
	}

	// Sent when all work has completed.
	EventDone struct {
		Err error
	}
)
