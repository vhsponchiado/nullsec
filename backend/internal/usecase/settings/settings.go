// Package settings is the "configure the upstream proxy" use case — the
// SOCKS5/HTTP forward proxy that Repeater, Intruder, the Port Scanner, and
// the Subdomain Finder's crt.sh lookup route their traffic through. The
// only implementation of UpstreamConfig is infra/upstream.Manager, but this
// use case doesn't know that.
package settings

import "nullsec/backend/internal/entity"

type UpstreamConfig interface {
	Get() entity.UpstreamProxyConfig
	Set(cfg entity.UpstreamProxyConfig) error
}

type UseCase struct {
	upstream UpstreamConfig
}

func New(upstream UpstreamConfig) *UseCase {
	return &UseCase{upstream: upstream}
}

func (u *UseCase) GetUpstreamProxy() entity.UpstreamProxyConfig {
	return u.upstream.Get()
}

func (u *UseCase) SetUpstreamProxy(cfg entity.UpstreamProxyConfig) (entity.UpstreamProxyConfig, error) {
	if err := u.upstream.Set(cfg); err != nil {
		return entity.UpstreamProxyConfig{}, err
	}
	return u.upstream.Get(), nil
}
