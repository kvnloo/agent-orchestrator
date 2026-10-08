package report

import (
	"context"
	"errors"
)

// errReportBatchDeferred marks a delivery failure whose claimed batch was
// durably moved behind the coordinator retry delay. RunDue may continue with
// the next due batch without re-claiming the same failed batch in a tight loop.
var errReportBatchDeferred = errors.New("report delivery batch deferred")

func (c *Coordinator) deferFailure(ctx context.Context, batch PreparedBatch, cause error) error {
	retryAt := c.now().UTC().Add(c.retryDelay)
	_, deferErr := c.store.DeferReportBatch(
		ctx,
		batch.ProjectID,
		batch.Token,
		retryAt,
		cause.Error(),
	)
	if deferErr != nil {
		return errors.Join(cause, deferErr)
	}
	return errors.Join(cause, errReportBatchDeferred)
}

func joinReportDeliveryErrors(deferred []error, current error) error {
	if current != nil {
		deferred = append(deferred, current)
	}
	return errors.Join(deferred...)
}
