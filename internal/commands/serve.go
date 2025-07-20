package commands

import (
	"context"
)

// Default ports for okra serve
const (
	defaultServicePort = 8080
	defaultAdminPort   = 8081
)

// ServeOptions contains options for the serve command
type ServeOptions struct {
	ServicePort int
	AdminPort   int
}

func (c *Controller) Serve(ctx context.Context, opts ...ServeOptions) error {
	// Convert to single ServeOptions
	serveOpts := ServeOptions{
		ServicePort: defaultServicePort,
		AdminPort:   defaultAdminPort,
	}
	
	if len(opts) > 0 {
		if opts[0].ServicePort > 0 {
			serveOpts.ServicePort = opts[0].ServicePort
		}
		if opts[0].AdminPort > 0 {
			serveOpts.AdminPort = opts[0].AdminPort
		}
	}
	
	// Use the refactored command
	cmd := NewServeCommand()
	return cmd.Execute(ctx, serveOpts)
}
