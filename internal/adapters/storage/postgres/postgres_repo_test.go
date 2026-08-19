package postgres

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"splitle/internal/core/domain"
)

func d(val float64) decimal.Decimal {
	return decimal.NewFromFloat(val)
}

func getMigrationPath() string {
	_, filename, _, _ := runtime.Caller(0)
	// From internal/adapters/storage/postgres/ to migrations/000001_init_schema.up.sql
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "migrations", "000001_init_schema.up.sql")
}

func setupTestcontainersPostgres(t *testing.T) (*PostgresRepository, func()) {
	t.Helper()
	ctx := context.Background()

	migrationFile := getMigrationPath()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithInitScripts(migrationFile),
		tcpostgres.WithDatabase("splitle_test_db"),
		tcpostgres.WithUsername("splitle_test"),
		tcpostgres.WithPassword("splitle_test_pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Skipf("Docker or Testcontainers unavailable, skipping test: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	repo, err := NewPostgresRepositoryFromURL(ctx, connStr)
	require.NoError(t, err)

	cleanup := func() {
		repo.Close()
		_ = testcontainers.TerminateContainer(pgContainer)
	}

	return repo, cleanup
}

func TestPostgresRepository_Testcontainers(t *testing.T) {
	repo, cleanup := setupTestcontainersPostgres(t)
	defer cleanup()

	ctx := context.Background()
	groupID := "tg-group-" + uuid.New().String()

	t.Run("create and retrieve transaction with entries", func(t *testing.T) {
		tx := &domain.Transaction{
			ID:           uuid.New(),
			GroupID:      groupID,
			Description:  "Tokyo Team Dinner",
			Currency:     "EUR",
			BaseCurrency: "EUR",
			ExchangeRate: decimal.NewFromInt(1),
			CreatedBy:    "alice",
			CreatedAt:    time.Now().UTC(),
			Entries: []domain.Entry{
				{
					ID:         uuid.New(),
					UserID:     "alice",
					PaidAmount: d(90.00),
					OwedAmount: d(30.00),
				},
				{
					ID:         uuid.New(),
					UserID:     "bob",
					PaidAmount: decimal.Zero,
					OwedAmount: d(30.00),
				},
				{
					ID:         uuid.New(),
					UserID:     "charlie",
					PaidAmount: decimal.Zero,
					OwedAmount: d(30.00),
				},
			},
		}

		err := repo.CreateTransaction(ctx, tx)
		require.NoError(t, err)

		// 1. Get by ID
		fetched, err := repo.GetTransactionByID(ctx, tx.ID)
		require.NoError(t, err)
		assert.Equal(t, tx.ID, fetched.ID)
		assert.Equal(t, "Tokyo Team Dinner", fetched.Description)
		assert.Equal(t, "EUR", fetched.Currency)
		assert.True(t, decimal.NewFromInt(1).Equal(fetched.ExchangeRate))
		assert.Len(t, fetched.Entries, 3)

		// 2. Get by Group
		groupTxs, err := repo.GetTransactionsByGroup(ctx, groupID)
		require.NoError(t, err)
		assert.Len(t, groupTxs, 1)
		assert.Equal(t, tx.ID, groupTxs[0].ID)
		assert.Len(t, groupTxs[0].Entries, 3)

		// 3. Soft Delete
		err = repo.SoftDeleteTransaction(ctx, tx.ID)
		require.NoError(t, err)

		// Deleted transaction should no longer be returned by GetByID or GetByGroup
		_, err = repo.GetTransactionByID(ctx, tx.ID)
		assert.ErrorIs(t, err, domain.ErrTransactionNotFound)

		groupTxsAfter, err := repo.GetTransactionsByGroup(ctx, groupID)
		require.NoError(t, err)
		assert.Empty(t, groupTxsAfter)
	})

	t.Run("database unique constraint on duplicate user in same transaction", func(t *testing.T) {
		txID := uuid.New()
		tx := &domain.Transaction{
			ID:           txID,
			GroupID:      groupID,
			Description:  "Invalid duplicate entries",
			Currency:     "EUR",
			BaseCurrency: "EUR",
			ExchangeRate: decimal.NewFromInt(1),
			CreatedBy:    "alice",
			CreatedAt:    time.Now().UTC(),
			Entries: []domain.Entry{
				{
					ID:         uuid.New(),
					UserID:     "alice",
					PaidAmount: d(50.00),
					OwedAmount: decimal.Zero,
				},
				{
					ID:         uuid.New(),
					UserID:     "alice", // Duplicate user ID in same transaction
					PaidAmount: decimal.Zero,
					OwedAmount: d(50.00),
				},
			},
		}

		// In-domain validation will catch duplicate users
		err := repo.CreateTransaction(ctx, tx)
		assert.ErrorIs(t, err, domain.ErrDuplicateUserInEntries)
	})

	t.Run("ping verifies connection", func(t *testing.T) {
		err := repo.Ping(ctx)
		assert.NoError(t, err)
	})
}
