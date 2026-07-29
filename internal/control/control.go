package control

import "context"

// Service exposes runtime lifecycle operations to an OpenWrt control adapter.
type Service interface {
	Reload(context.Context) error
	Status(context.Context) (Status, error)
}

type Status struct {
	Running bool `json:"running"`
}
