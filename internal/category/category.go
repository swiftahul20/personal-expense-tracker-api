package category

import (
	"errors"
	"strings"
)

type Category struct {
	ID            int           `json:"id"`
	Name          string        `json:"name"`
	SubCategories []SubCategory `json:"sub_categories"`
}

type SubCategory struct {
	ID         int    `json:"id"`
	CategoryID int    `json:"category_id"`
	Name       string `json:"name"`
}

var (
	ErrEmptyName           = errors.New("name is required")
	ErrDuplicateName       = errors.New("a category with this name already exists")
	ErrCategoryNotFound    = errors.New("category not found")
	ErrSubCategoryNotFound = errors.New("sub-category not found")
)

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrEmptyName
	}
	return name, nil
}

type Store interface {
	Create(userID int, name string) (Category, error)
	List(userID int) ([]Category, error)
	Update(userID, id int, name string) (Category, error)
	Delete(userID, id int) error

	CreateSubCategory(userID, categoryID int, name string) (SubCategory, error)
	UpdateSubCategory(userID, id int, name string) (SubCategory, error)
	DeleteSubCategory(userID, id int) error
}
