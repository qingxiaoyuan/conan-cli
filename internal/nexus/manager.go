package nexus

import (
	"context"

	"conan-cli/internal/conan"
)

type Manager struct {
	Client *conan.Client
}

func (m Manager) List(ctx context.Context) (conan.Result, error) {
	return m.Client.Remotes(ctx)
}

func (m Manager) Add(ctx context.Context, name, url string) (conan.Result, error) {
	return m.Client.RemoteAdd(ctx, name, url)
}

func (m Manager) Login(ctx context.Context, name, username, password string) (conan.Result, error) {
	return m.Client.RemoteLogin(ctx, name, username, password)
}
