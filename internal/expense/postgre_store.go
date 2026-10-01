package expense

import (
	"context"
	"fmt"

	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ListParams struct {
	Page       int
	Limit      int
	CategoryID *int
	Search     string
	From       *time.Time
	To         *time.Time
}

type ListResult struct {
	Expenses []Expense
	Total    int
}

type ExpenseUpdate struct {
	Amount        *float64
	CategoryID    *int
	SubCategoryID *int
	Description   *string
}

type Store interface {
	Add(userID int, e Expense) (Expense, error)
	List(userID int, params ListParams) (ListResult, error)
	ListAll(userID int) ([]Expense, error)
	ExportAll(userID int, params ListParams) ([]Expense, error)
	GetByID(userID, id int) (Expense, error)
	Delete(userID, id int) error
	Update(userID, id int, updates ExpenseUpdate) (Expense, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func buildFilterQuery(userID int, params ListParams) (where string, args []interface{}, nextArgN int) {
	where = "WHERE e.user_id = $1"
	args = []interface{}{userID}
	argN := 2

	if params.CategoryID != nil {
		where += fmt.Sprintf(" AND e.category_id = $%d", argN)
		args = append(args, *params.CategoryID)
		argN++
	}
	if params.Search != "" {
		where += fmt.Sprintf(" AND e.description ILIKE $%d", argN)
		args = append(args, "%"+params.Search+"%")
		argN++
	}
	if params.From != nil {
		where += fmt.Sprintf(" AND e.date >= $%d", argN)
		args = append(args, *params.From)
		argN++
	}
	if params.To != nil {
		where += fmt.Sprintf(" AND e.date <= $%d", argN)
		args = append(args, *params.To)
		argN++
	}

	return where, args, argN
}

const expenseSelectAndJoin = `
	SELECT e.id, e.amount, e.category_id, c.name, e.sub_category_id, sc.name, e.description, e.date
	FROM expenses e
	LEFT JOIN categories c ON c.id = e.category_id
	LEFT JOIN sub_categories sc ON sc.id = e.sub_category_id
`

func (s *PostgresStore) Add(userID int, e Expense) (Expense, error) {
	if err := e.Validate(); err != nil {
		return Expense{}, err
	}

	ctx := context.Background()
	query := `INSERT INTO expenses (user_id, amount, category_id, sub_category_id, description, date)
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	err := s.pool.QueryRow(ctx, query, userID, e.Amount, e.CategoryID, e.SubCategoryID, e.Description, e.Date).Scan(&e.ID)
	if err != nil {
		return Expense{}, fmt.Errorf("inserting expense: %w", err)
	}

	return s.GetByID(userID, e.ID)
}

func (s *PostgresStore) List(userID int, params ListParams) (ListResult, error) {
	ctx := context.Background()

	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	where, args, argN := buildFilterQuery(userID, params)

	var total int
	countQuery := "SELECT COUNT(*) FROM expenses e " + where
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return ListResult{}, fmt.Errorf("counting expenses: %w", err)
	}

	query := fmt.Sprintf(
		expenseSelectAndJoin+" %s ORDER BY e.date DESC LIMIT $%d OFFSET $%d",
		where, argN, argN+1,
	)
	args = append(args, params.Limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("querying expenses: %w", err)
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		var e Expense
		var categoryName, subCategoryName *string
		if err := rows.Scan(&e.ID, &e.Amount, &e.CategoryID, &categoryName, &e.SubCategoryID, &subCategoryName, &e.Description, &e.Date); err != nil {
			return ListResult{}, fmt.Errorf("scanning expense row: %w", err)
		}
		if categoryName != nil {
			e.CategoryName = *categoryName
		}
		if subCategoryName != nil {
			e.SubCategoryName = *subCategoryName
		}
		expenses = append(expenses, e)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}

	return ListResult{Expenses: expenses, Total: total}, nil
}

func (s *PostgresStore) ListAll(userID int) ([]Expense, error) {
	ctx := context.Background()
	query := expenseSelectAndJoin + " WHERE e.user_id = $1 ORDER BY e.date"

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying expenses: %w", err)
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		var e Expense
		var categoryName, subCategoryName *string
		if err := rows.Scan(&e.ID, &e.Amount, &e.CategoryID, &categoryName, &e.SubCategoryID, &subCategoryName, &e.Description, &e.Date); err != nil {
			return nil, fmt.Errorf("scanning expense row: %w", err)
		}
		if categoryName != nil {
			e.CategoryName = *categoryName
		}
		if subCategoryName != nil {
			e.SubCategoryName = *subCategoryName
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

func (s *PostgresStore) ExportAll(userID int, params ListParams) ([]Expense, error) {
	ctx := context.Background()

	where, args, _ := buildFilterQuery(userID, params)
	query := expenseSelectAndJoin + " " + where + " ORDER BY e.date DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying expenses: %w", err)
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		var e Expense
		var categoryName, subCategoryName *string
		if err := rows.Scan(&e.ID, &e.Amount, &e.CategoryID, &categoryName, &e.SubCategoryID, &subCategoryName, &e.Description, &e.Date); err != nil {
			return nil, fmt.Errorf("scanning expense row: %w", err)
		}
		if categoryName != nil {
			e.CategoryName = *categoryName
		}
		if subCategoryName != nil {
			e.SubCategoryName = *subCategoryName
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

func (s *PostgresStore) GetByID(userID, id int) (Expense, error) {
	ctx := context.Background()
	query := expenseSelectAndJoin + " WHERE e.id = $1 AND e.user_id = $2"

	var e Expense
	var categoryName, subCategoryName *string
	err := s.pool.QueryRow(ctx, query, id, userID).Scan(
		&e.ID, &e.Amount, &e.CategoryID, &categoryName, &e.SubCategoryID, &subCategoryName, &e.Description, &e.Date,
	)
	if err != nil {
		return Expense{}, fmt.Errorf("expense with id %d not found", id)
	}
	if categoryName != nil {
		e.CategoryName = *categoryName
	}
	if subCategoryName != nil {
		e.SubCategoryName = *subCategoryName
	}
	return e, nil
}

func (s *PostgresStore) Delete(userID, id int) error {
	ctx := context.Background()
	tag, err := s.pool.Exec(ctx, `DELETE FROM expenses WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting expense: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("expense with id %d not found", id)
	}
	return nil
}

func (s *PostgresStore) Update(userID, id int, updates ExpenseUpdate) (Expense, error) {
	ctx := context.Background()

	current, err := s.GetByID(userID, id)
	if err != nil {
		return Expense{}, err
	}

	if updates.Amount != nil {
		current.Amount = *updates.Amount
	}
	if updates.CategoryID != nil {
		current.CategoryID = updates.CategoryID
	}
	if updates.SubCategoryID != nil {
		current.SubCategoryID = updates.SubCategoryID
	}
	if updates.Description != nil {
		current.Description = *updates.Description
	}

	if err := current.Validate(); err != nil {
		return Expense{}, err
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE expenses SET amount = $1, category_id = $2, sub_category_id = $3, description = $4 WHERE id = $5 AND user_id = $6`,
		current.Amount, current.CategoryID, current.SubCategoryID, current.Description, id, userID,
	)
	if err != nil {
		return Expense{}, fmt.Errorf("updating expense: %w", err)
	}

	return s.GetByID(userID, id)
}

var _ = pgx.ErrNoRows // keep pgx import if unused elsewhere; remove this line if pgx.ErrNoRows is used below
