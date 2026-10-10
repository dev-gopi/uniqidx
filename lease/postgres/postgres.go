// Package postgres provides a durable PostgreSQL LeaseProvider.
package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/dev-gopi/uniqidx"
)

var ErrLeaseLost = errors.New("uniqidx/lease/postgres: lease is no longer active")

type Provider struct {
	db     *sql.DB
	ttl    time.Duration
	random io.Reader
}

func New(db *sql.DB, ttl time.Duration, random io.Reader) (*Provider, error) {
	if db == nil {
		return nil, errors.New("uniqidx/lease/postgres: nil database")
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if random == nil {
		random = rand.Reader
	}
	return &Provider{db: db, ttl: ttl, random: random}, nil
}

// Migrate creates the permanent allocation registry and active lease table.
// Run it through your ordinary migration process in production.
func (p *Provider) Migrate(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS uniqidx_token_registry (token BYTEA PRIMARY KEY CHECK (octet_length(token)=9), allocated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP); CREATE TABLE IF NOT EXISTS uniqidx_active_leases (token BYTEA PRIMARY KEY REFERENCES uniqidx_token_registry(token), lease_id BYTEA NOT NULL UNIQUE CHECK (octet_length(lease_id)=16), expires_at TIMESTAMPTZ NOT NULL);`)
	return err
}
func tokenBytes(t uniqidx.Token69) []byte {
	b := make([]byte, 9)
	b[0] = t.Hi
	binary.BigEndian.PutUint64(b[1:], t.Lo)
	return b
}
func (p *Provider) Acquire(ctx context.Context) (uniqidx.Lease, error) {
	for attempts := 0; attempts < 1024; attempts++ {
		var b [9]byte
		var id [16]byte
		if _, e := io.ReadFull(p.random, b[:]); e != nil {
			return uniqidx.Lease{}, e
		}
		if _, e := io.ReadFull(p.random, id[:]); e != nil {
			return uniqidx.Lease{}, e
		}
		token := uniqidx.Token69{Hi: b[0] & 31, Lo: binary.BigEndian.Uint64(b[1:])}
		raw := tokenBytes(token)
		expires := time.Now().UTC().Add(p.ttl)
		tx, e := p.db.BeginTx(ctx, nil)
		if e != nil {
			return uniqidx.Lease{}, e
		}
		result, e := tx.ExecContext(ctx, `INSERT INTO uniqidx_token_registry(token) VALUES ($1) ON CONFLICT DO NOTHING`, raw)
		if e == nil {
			n, _ := result.RowsAffected()
			if n == 1 {
				_, e = tx.ExecContext(ctx, `INSERT INTO uniqidx_active_leases(token,lease_id,expires_at) VALUES ($1,$2,$3)`, raw, id[:], expires)
				if e == nil {
					e = tx.Commit()
					if e == nil {
						return uniqidx.Lease{Token: token, ID: id, ExpiresAt: expires}, nil
					}
				}
			}
		}
		_ = tx.Rollback()
		if e != nil {
			return uniqidx.Lease{}, e
		}
	}
	return uniqidx.Lease{}, errors.New("uniqidx/lease/postgres: random token allocation exhausted")
}
func (p *Provider) Renew(ctx context.Context, l uniqidx.Lease) error {
	_, e := p.RenewLease(ctx, l)
	return e
}
func (p *Provider) RenewLease(ctx context.Context, l uniqidx.Lease) (uniqidx.Lease, error) {
	expires := time.Now().UTC().Add(p.ttl)
	r, e := p.db.ExecContext(ctx, `UPDATE uniqidx_active_leases SET expires_at=$3 WHERE token=$1 AND lease_id=$2 AND expires_at>CURRENT_TIMESTAMP`, tokenBytes(l.Token), l.ID[:], expires)
	if e != nil {
		return uniqidx.Lease{}, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return uniqidx.Lease{}, e
	}
	if n != 1 {
		return uniqidx.Lease{}, ErrLeaseLost
	}
	l.ExpiresAt = expires
	return l, nil
}
func (p *Provider) Release(ctx context.Context, l uniqidx.Lease) error {
	_, e := p.db.ExecContext(ctx, `DELETE FROM uniqidx_active_leases WHERE token=$1 AND lease_id=$2`, tokenBytes(l.Token), l.ID[:])
	return e
}
