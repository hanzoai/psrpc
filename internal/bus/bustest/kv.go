package bustest

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hanzokv/go/v9"
	"github.com/ory/dockertest/v4"

	"github.com/hanzoai/psrpc/internal/bus"
)

func init() {
	RegisterServer("KV", NewKV)
}

var kvLast = baseID

func NewKV(t testing.TB, pool dockertest.Pool) Server {
	ctx := context.Background()
	c, err := pool.Run(ctx, "redis",
		dockertest.WithTag("latest"),
		dockertest.WithName(fmt.Sprintf("psrpc-kv-%d", atomic.AddUint32(&kvLast, 1))),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.Close(context.Background())
	})
	addr := c.GetHostPort("6379/tcp")
	waitTCPPort(t, pool, addr)

	t.Log("KV running on", addr)

	s := &kvServer{addr: addr}

	err = pool.Retry(ctx, 0, func() error {
		rc, err := s.connect()
		if err != nil {
			return err
		}
		_ = rc.Close()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return s
}

type kvServer struct {
	addr string
}

func (s *kvServer) connect() (kv.UniversalClient, error) {
	rc := kv.NewUniversalClient(&kv.UniversalOptions{Addrs: []string{s.addr}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := rc.Ping(ctx).Err(); err != nil {
		_ = rc.Close()
		return nil, err
	}

	return rc, nil
}

func (s *kvServer) Connect(t testing.TB) bus.MessageBus {
	rc, err := s.connect()
	if err != nil {
		t.Fatal(err)
	}
	return bus.NewKVMessageBus(rc)
}
