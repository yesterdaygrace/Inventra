// Package audit defines a leaf audit-logging interface that mutation
// modules depend on. The concrete implementation lives in activitylog,
// which is wired in at composition time; defining the interface here keeps
// auth/product/category/inventory free of an import cycle back to it.
package audit

import "github.com/google/uuid"

// Entry describes a single audit event to be recorded.
type Entry struct {
	UserID     *uuid.UUID
	Action     string
	EntityType string
	EntityID   *string
	Details    any
	IP         *string
}

// Recorder persists audit events. Implementations (activitylog.Service)
// are required to be failure-safe: a recording error must never fail the
// business operation that produced the event, so Record returns nothing.
type Recorder interface {
	Record(e Entry)
}

// Nop is a no-op recorder used where audit wiring is optional (nil-safe).
type Nop struct{}

// Record ignores the event.
func (Nop) Record(Entry) {}

var _ Recorder = Nop{}