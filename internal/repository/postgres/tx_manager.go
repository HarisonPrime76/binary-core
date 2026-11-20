package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxManager définit le contrat pour exécuter des opérations dans une transaction ACID.
type TxManager interface {
	ExecTx(ctx context.Context, fn func(q Querier) error) error
}

type PostgresTxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *PostgresTxManager {
	return &PostgresTxManager{pool: pool}
}

// ExecTx démarre une transaction PostgreSQL, exécute la fonction métier,
// et garantit un Rollback en cas d'erreur ou de panic.
func (tm *PostgresTxManager) ExecTx(ctx context.Context, fn func(q Querier) error) error {
	tx, err := tm.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	// Création d'une instance SQLC Queries liée à la transaction en cours
	qtx := New(tx)

	if err := fn(qtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err: %v, rollback err: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
