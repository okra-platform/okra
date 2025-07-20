package commands

import (
	"context"
)

// Dev runs the development server with hot-reloading for OKRA services
func (c *Controller) Dev(ctx context.Context) error {
	// Use the refactored command
	cmd := NewDevCommand()
	return cmd.Execute(ctx)
}
