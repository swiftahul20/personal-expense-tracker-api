package category

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(userID int, name string) (Category, error) {
	name, err := ValidateName(name)
	if err != nil {
		return Category{}, err
	}

	ctx := context.Background()
	var c Category
	err = s.pool.QueryRow(ctx,
		`INSERT INTO categories (user_id, name) VALUES ($1, $2) RETURNING id, name`,
		userID, name,
	).Scan(&c.ID, &c.Name)
	if err != nil {
		return Category{}, ErrDuplicateName
	}
	c.SubCategories = []SubCategory{}
	return c, nil
}

func (s *PostgresStore) List(userID int) ([]Category, error) {
	ctx := context.Background()

	rows, err := s.pool.Query(ctx,
		`SELECT id, name FROM categories WHERE user_id = $1 ORDER BY name`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()

	categories := make(map[int]*Category)
	var order []int

	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		c.SubCategories = []SubCategory{}
		categories[c.ID] = &c
		order = append(order, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(order) > 0 {
		subRows, err := s.pool.Query(ctx,
			`SELECT sc.id, sc.category_id, sc.name FROM sub_categories sc
			 JOIN categories c ON c.id = sc.category_id
			 WHERE c.user_id = $1 ORDER BY sc.name`, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("querying sub-categories: %w", err)
		}
		defer subRows.Close()

		for subRows.Next() {
			var sc SubCategory
			if err := subRows.Scan(&sc.ID, &sc.CategoryID, &sc.Name); err != nil {
				return nil, fmt.Errorf("scanning sub-category: %w", err)
			}
			if c, ok := categories[sc.CategoryID]; ok {
				c.SubCategories = append(c.SubCategories, sc)
			}
		}
		if err := subRows.Err(); err != nil {
			return nil, err
		}
	}

	result := make([]Category, 0, len(order))
	for _, id := range order {
		result = append(result, *categories[id])
	}
	return result, nil
}

func (s *PostgresStore) Update(userID, id int, name string) (Category, error) {
	name, err := ValidateName(name)
	if err != nil {
		return Category{}, err
	}

	ctx := context.Background()
	var c Category
	err = s.pool.QueryRow(ctx,
		`UPDATE categories SET name = $1 WHERE id = $2 AND user_id = $3 RETURNING id, name`,
		name, id, userID,
	).Scan(&c.ID, &c.Name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Category{}, ErrCategoryNotFound
		}
		return Category{}, ErrDuplicateName
	}
	return c, nil
}

func (s *PostgresStore) Delete(userID, id int) error {
	ctx := context.Background()
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM categories WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func (s *PostgresStore) CreateSubCategory(userID, categoryID int, name string) (SubCategory, error) {
	name, err := ValidateName(name)
	if err != nil {
		return SubCategory{}, err
	}

	ctx := context.Background()

	var ownerCheck int
	err = s.pool.QueryRow(ctx,
		`SELECT id FROM categories WHERE id = $1 AND user_id = $2`, categoryID, userID,
	).Scan(&ownerCheck)
	if err != nil {
		return SubCategory{}, ErrCategoryNotFound
	}

	var sc SubCategory
	err = s.pool.QueryRow(ctx,
		`INSERT INTO sub_categories (category_id, name) VALUES ($1, $2) RETURNING id, category_id, name`,
		categoryID, name,
	).Scan(&sc.ID, &sc.CategoryID, &sc.Name)
	if err != nil {
		return SubCategory{}, ErrDuplicateName
	}
	return sc, nil
}

func (s *PostgresStore) UpdateSubCategory(userID, id int, name string) (SubCategory, error) {
	name, err := ValidateName(name)
	if err != nil {
		return SubCategory{}, err
	}

	ctx := context.Background()
	var sc SubCategory
	err = s.pool.QueryRow(ctx,
		`UPDATE sub_categories sc SET name = $1
		 FROM categories c
		 WHERE sc.id = $2 AND sc.category_id = c.id AND c.user_id = $3
		 RETURNING sc.id, sc.category_id, sc.name`,
		name, id, userID,
	).Scan(&sc.ID, &sc.CategoryID, &sc.Name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return SubCategory{}, ErrSubCategoryNotFound
		}
		return SubCategory{}, ErrDuplicateName
	}
	return sc, nil
}

func (s *PostgresStore) DeleteSubCategory(userID, id int) error {
	ctx := context.Background()
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM sub_categories sc USING categories c
		 WHERE sc.id = $1 AND sc.category_id = c.id AND c.user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting sub-category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSubCategoryNotFound
	}
	return nil
}
