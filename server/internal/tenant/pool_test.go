package tenant

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

type failingDriver struct{}

func (f failingDriver) Open(name string) (driver.Conn, error) {
	return failingConn{}, nil
}

type failingConn struct{}

func (f failingConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (f failingConn) Close() error {
	return nil
}

func (f failingConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not supported")
}

func (f failingConn) Ping(ctx context.Context) error {
	return errors.New("ping failed")
}

func init() {
	sql.Register("mysql", failingDriver{})
}

func TestNewPool_Defaults(t *testing.T) {
	pool := NewPool(PoolConfig{})
	defer pool.Close()

	if pool.maxIdle != 5 {
		t.Fatalf("maxIdle = %d, want %d", pool.maxIdle, 5)
	}
	if pool.maxOpen != 10 {
		t.Fatalf("maxOpen = %d, want %d", pool.maxOpen, 10)
	}
	if pool.lifetime != 30*time.Minute {
		t.Fatalf("lifetime = %v, want %v", pool.lifetime, 30*time.Minute)
	}
	if pool.idleTimeout != 10*time.Minute {
		t.Fatalf("idleTimeout = %v, want %v", pool.idleTimeout, 10*time.Minute)
	}
	if pool.totalLimit != 200 {
		t.Fatalf("totalLimit = %d, want %d", pool.totalLimit, 200)
	}
}

func TestNewPool_CustomConfig(t *testing.T) {
	cfg := PoolConfig{
		MaxIdle:     2,
		MaxOpen:     4,
		Lifetime:    15 * time.Minute,
		IdleTimeout: 5 * time.Minute,
		TotalLimit:  9,
	}
	pool := NewPool(cfg)
	defer pool.Close()

	if pool.maxIdle != cfg.MaxIdle {
		t.Fatalf("maxIdle = %d, want %d", pool.maxIdle, cfg.MaxIdle)
	}
	if pool.maxOpen != cfg.MaxOpen {
		t.Fatalf("maxOpen = %d, want %d", pool.maxOpen, cfg.MaxOpen)
	}
	if pool.lifetime != cfg.Lifetime {
		t.Fatalf("lifetime = %v, want %v", pool.lifetime, cfg.Lifetime)
	}
	if pool.idleTimeout != cfg.IdleTimeout {
		t.Fatalf("idleTimeout = %v, want %v", pool.idleTimeout, cfg.IdleTimeout)
	}
	if pool.totalLimit != cfg.TotalLimit {
		t.Fatalf("totalLimit = %d, want %d", pool.totalLimit, cfg.TotalLimit)
	}
}

func TestPool_Get_InvalidDSN(t *testing.T) {
	pool := NewPool(PoolConfig{})
	defer pool.Close()

	dsn := "user:pass@tcp(127.0.0.1:1)/db?parseTime=true"
	_, err := pool.Get(context.Background(), "tenant-1", dsn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPool_Remove_NonExistent(t *testing.T) {
	pool := NewPool(PoolConfig{})
	defer pool.Close()

	pool.Remove("missing-tenant")
}

func TestPool_Stats_Empty(t *testing.T) {
	pool := NewPool(PoolConfig{})
	defer pool.Close()

	stats := pool.Stats()
	if len(stats) != 0 {
		t.Fatalf("expected empty stats, got %d", len(stats))
	}
}

func TestPool_Close_Idempotent(t *testing.T) {
	pool := NewPool(PoolConfig{})
	pool.Close()

	dsn := "user:pass@tcp(127.0.0.1:1)/db?parseTime=true"
	_, err := pool.Get(context.Background(), "tenant-1", dsn)
	if err == nil {
		t.Fatal("expected error after Close, got nil")
	}
}

func TestPool_TotalLimit(t *testing.T) {
	pool := NewPool(PoolConfig{TotalLimit: 1})
	defer pool.Close()

	db, err := sql.Open("mysql", "user:pass@tcp(127.0.0.1:1)/db?parseTime=true")
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}

	pool.mu.Lock()
	pool.conns["tenant-1"] = &tenantConn{db: db, lastUsed: time.Now(), tenantID: "tenant-1"}
	pool.mu.Unlock()

	_, err = pool.Get(context.Background(), "tenant-2", "user:pass@tcp(127.0.0.1:1)/db?parseTime=true")
	if err == nil {
		t.Fatal("expected total limit error, got nil")
	}
	if !strings.Contains(err.Error(), "total limit 1 reached") {
		t.Fatalf("unexpected error: %v", err)
	}
}
