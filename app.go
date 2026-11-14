package main

import (
	"context"

	"github.com/reeschan/atelier-desk/internal/desk"
)

// App owns the lifecycle and hands it to the Service. App itself is not bound
// to the screen; only the Service is.
type App struct {
	ctx context.Context
	svc *desk.Service
}

// NewApp creates a new App around svc.
func NewApp(svc *desk.Service) *App {
	return &App{svc: svc}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	desk.Start(a.svc, ctx)
}

func (a *App) shutdown(ctx context.Context) {
	desk.Stop(a.svc, ctx)
}
