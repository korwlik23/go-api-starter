package app

import "context"

// Run owns the API process lifecycle. Platform dependencies and the HTTP server
// are composed here as their milestones are implemented.
func Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
