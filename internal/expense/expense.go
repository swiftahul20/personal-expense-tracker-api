package expense

import (
	"errors"
	"strings"
	"time"
)

type Expense struct {
	ID              int       `json:"id"`
	Amount          float64   `json:"amount"`
	CategoryID      *int      `json:"category_id"`
	CategoryName    string    `json:"category_name,omitempty"`
	SubCategoryID   *int      `json:"sub_category_id"`
	SubCategoryName string    `json:"sub_category_name,omitempty"`
	Description     string    `json:"description"`
	Date            time.Time `json:"date"`
}

var (
	ErrInvalidAmount = errors.New("amount must be greater than 0")
	ErrEmptyCategory = errors.New("category is required")
	ErrFutureDate    = errors.New("date cannot be in the future")
)

func (e *Expense) Validate() error {
	if e.Amount <= 0 {
		return ErrInvalidAmount
	}

	if e.CategoryID == nil {
		return ErrEmptyCategory
	}

	e.Description = strings.TrimSpace(e.Description)

	if e.Date.After(time.Now().Add(24 * time.Hour)) {
		return ErrFutureDate
	}

	return nil
}