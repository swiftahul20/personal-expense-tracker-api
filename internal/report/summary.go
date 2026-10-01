package report

import (
	"sort"

	"github.com/swiftahul20/expense-tracker/internal/expense"
)

// types
type CategoryTotal struct {
	Category string            `json:"category"`
	Total    float64           `json:"total"`
	Expenses []expense.Expense `json:"expenses"`
}

type MonthTotal struct {
	Month    string            `json:"month"`
	Total    float64           `json:"total"`
	Expenses []expense.Expense `json:"expenses"`
}

type DayTotal struct {
	Day      string            `json:"day"`
	Total    float64           `json:"total"`
	Expenses []expense.Expense `json:"expenses"`
}

// ================================

func ByCategory(expenses []expense.Expense) []CategoryTotal {
	groups := make(map[string][]expense.Expense)
	for _, e := range expenses {
		key := e.CategoryName
		if key == "" {
			key = "Uncategorized"
		}
		groups[key] = append(groups[key], e)
	}

	result := make([]CategoryTotal, 0, len(groups))
	for category, items := range groups {
		var total float64
		for _, e := range items {
			total += e.Amount
		}
		result = append(result, CategoryTotal{Category: category, Total: total, Expenses: items})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Total > result[j].Total
	})

	return result
}

func ByMonth(expenses []expense.Expense) []MonthTotal {
	groups := make(map[string][]expense.Expense)
	for _, e := range expenses {
		key := e.Date.Format("2006-01")
		groups[key] = append(groups[key], e)
	}

	result := make([]MonthTotal, 0, len(groups))
	for month, items := range groups {
		var total float64
		for _, e := range items {
			total += e.Amount
		}
		result = append(result, MonthTotal{Month: month, Total: total, Expenses: items})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Month < result[j].Month
	})

	return result
}

func ByDay(expenses []expense.Expense) []DayTotal {
	groups := make(map[string][]expense.Expense)
	for _, e := range expenses {
		key := e.Date.Format("2006-01-02")
		groups[key] = append(groups[key], e)
	}

	result := make([]DayTotal, 0, len(groups))
	for day, items := range groups {
		var total float64
		for _, e := range items {
			total += e.Amount
		}
		result = append(result, DayTotal{Day: day, Total: total, Expenses: items})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Day < result[j].Day
	})

	return result
}
