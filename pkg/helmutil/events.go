package helmutil

type (
	// Sent when a chart has been added.
	EventAddedChart struct {
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
)
