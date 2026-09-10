package ai

import (
	"context"

	"github.com/7AkhilV/daily-work/internal/activity"
	"github.com/7AkhilV/daily-work/internal/summary"
)

// ActivityInput is what we send to the AI layer.
type ActivityInput struct {
	Processed *activity.ProcessedActivity
	DateLabel string
}

// Provider abstracts AI backends.
type Provider interface {
	Summarize(ctx context.Context, input ActivityInput) ([]summary.WorkItem, error)
}
