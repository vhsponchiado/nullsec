package settings

import (
	"errors"
	"testing"

	"nullsec/backend/internal/entity"
)

type fakeUpstream struct {
	cfg    entity.UpstreamProxyConfig
	setErr error
}

func (f *fakeUpstream) Get() entity.UpstreamProxyConfig { return f.cfg }
func (f *fakeUpstream) Set(cfg entity.UpstreamProxyConfig) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.cfg = cfg
	return nil
}

func TestGetUpstreamProxy(t *testing.T) {
	want := entity.UpstreamProxyConfig{Type: entity.UpstreamSOCKS5, Address: "127.0.0.1:9050"}
	uc := New(&fakeUpstream{cfg: want})

	if got := uc.GetUpstreamProxy(); got != want {
		t.Fatalf("GetUpstreamProxy() = %+v, want %+v", got, want)
	}
}

func TestSetUpstreamProxy_Success(t *testing.T) {
	fake := &fakeUpstream{}
	uc := New(fake)

	want := entity.UpstreamProxyConfig{Type: entity.UpstreamHTTP, Address: "127.0.0.1:8888"}
	got, err := uc.SetUpstreamProxy(want)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("SetUpstreamProxy() = %+v, want %+v", got, want)
	}
	if fake.cfg != want {
		t.Fatalf("underlying config = %+v, want it updated to %+v", fake.cfg, want)
	}
}

func TestSetUpstreamProxy_PropagatesValidationError(t *testing.T) {
	wantErr := errors.New("invalid address")
	fake := &fakeUpstream{setErr: wantErr}
	uc := New(fake)

	_, err := uc.SetUpstreamProxy(entity.UpstreamProxyConfig{Type: entity.UpstreamSOCKS5})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
