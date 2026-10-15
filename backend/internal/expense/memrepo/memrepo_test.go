package memrepo_test

import (
	"testing"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
)

func TestRepositoryContract(t *testing.T) {
	expensetest.RunRepositoryContract(t, func(*testing.T) expense.Repository { return memrepo.New() })
}
