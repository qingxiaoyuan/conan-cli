package profile

import (
	"context"

	"conan-cli/internal/conan"
)

type Manager struct {
	Client *conan.Client
}

func (m Manager) List(ctx context.Context) (conan.Result, error) {
	return m.Client.Profiles(ctx)
}
