package rest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/swiftahul20/expense-tracker/internal/auth"
	"github.com/swiftahul20/expense-tracker/internal/expense"
	"github.com/swiftahul20/expense-tracker/internal/report"
)

// types
type Handler struct {
	store expense.Store
}

type paginatedExpensesResponse struct {
	Expenses   []expense.Expense `json:"expenses"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	Total      int               `json:"total"`
	TotalPages int               `json:"total_pages"`
}

type dashboardResponse struct {
	Expenses   []expense.Expense      `json:"expenses"`
	ByCategory []report.CategoryTotal `json:"by_category"`
	ByMonth    []report.MonthTotal    `json:"by_month"`
	ByDay      []report.DayTotal      `json:"by_day"`
}

type HealthHandler struct {
	pool *pgxpool.Pool
}

// =================================

// @Tags         Expenses
// @Produce      json
// @Security     BearerAuth
// @Param        page      query    int    false  "Page number (default 1)"
// @Param        limit     query    int    false  "Items per page (default 20, max 100)"
// @Param        category  query    string false  "Filter by category"
// @Param        search    query    string false  "Search by description (case-insensitive substring match)"
// @Param        from      query    string false  "Filter from this date (YYYY-MM-DD)"
// @Param        to        query    string false  "Filter up to this date (YYYY-MM-DD)"
// @Success      200 {object} paginatedExpensesResponse
// @Failure      401 {object} map[string]string
// @Router       /expenses [get]
func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	params := expense.ListParams{
		Page:   page,
		Limit:  limit,
		Search: r.URL.Query().Get("search"),
	}

	if catIDStr := r.URL.Query().Get("category_id"); catIDStr != "" {
		if catID, err := strconv.Atoi(catIDStr); err == nil {
			params.CategoryID = &catID
		}
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if from, err := time.Parse("2006-01-02", fromStr); err == nil {
			params.From = &from
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if to, err := time.Parse("2006-01-02", toStr); err == nil {
			to = to.Add(24*time.Hour - time.Nanosecond)
			params.To = &to
		}
	}

	result, err := h.store.List(userID, params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	page = params.Page
	if page < 1 {
		page = 1
	}
	limit = params.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	totalPages := (result.Total + limit - 1) / limit

	writeJSON(w, http.StatusOK, paginatedExpensesResponse{
		Expenses:   result.Expenses,
		Page:       page,
		Limit:      limit,
		Total:      result.Total,
		TotalPages: totalPages,
	})
}

func NewHandler(store expense.Store) *Handler {
	return &Handler{store: store}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// @Tags         Expenses
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Expense ID"
// @Success      200 {object} expense.Expense
// @Failure      401 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /expenses/{id} [get]
func (h *Handler) GetExpense(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	e, err := h.store.GetByID(userID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, e)
}

// @Tags         Expenses
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        expense body expense.Expense true "Expense to create"
// @Success      201 {object} expense.Expense
// @Failure      400 {object} map[string]string
// @Router       /expenses [post]
func (h *Handler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	var e expense.Expense
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.store.Add(userID, e)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// @Tags         Expenses
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Expense ID"
// @Param        updates body expense.ExpenseUpdate true "Fields to update"
// @Success      200 {object} expense.Expense
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Router       /expenses/{id} [put]
func (h *Handler) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var payload struct {
		Amount        *float64 `json:"amount"`
		CategoryID    *int     `json:"category_id"`
		SubCategoryID *int     `json:"sub_category_id"`
		Description   *string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := expense.ExpenseUpdate{
		Amount:        payload.Amount,
		CategoryID:    payload.CategoryID,
		SubCategoryID: payload.SubCategoryID,
		Description:   payload.Description,
	}

	updated, err := h.store.Update(userID, id, updates)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// @Tags         Expenses
// @Security     BearerAuth
// @Param        id path int true "Expense ID"
// @Success      204 "No Content"
// @Failure      401 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /expenses/{id} [delete]
func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.store.Delete(userID, id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Tags         Summary
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} report.CategoryTotal
// @Failure      401 {object} map[string]string
// @Router       /summary/category [get]
func (h *Handler) SummaryByCategory(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	expenses, err := h.store.ListAll(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report.ByCategory(expenses))
}

// @Tags         Summary
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} report.DayTotal
// @Failure      401 {object} map[string]string
// @Router       /summary/day [get]
func (h *Handler) SummaryByDay(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	expenses, err := h.store.ListAll(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report.ByDay(expenses))
}

// @Tags         Summary
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} report.MonthTotal
// @Failure      401 {object} map[string]string
// @Router       /summary/month [get]
func (h *Handler) SummaryByMonth(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	expenses, err := h.store.ListAll(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report.ByMonth(expenses))
}

// @Tags         Summary
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dashboardResponse
// @Failure      401 {object} map[string]string
// @Router       /dashboard [get]
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	expenses, err := h.store.ListAll(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		Expenses:   expenses,
		ByCategory: report.ByCategory(expenses),
		ByMonth:    report.ByMonth(expenses),
		ByDay:      report.ByDay(expenses),
	})
}

// @Tags         Health
// @Produce      json
// @Summary      Check API health
// @Description  Confirms the API is responsive and that PostgreSQL is reachable.
// @Success      200 {object} map[string]string "API and database are healthy"
// @Failure      503 {object} map[string]string "API is reachable but database is unavailable"
// @Router       /health [get]
func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "unhealthy",
			"database": "unreachable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "healthy",
		"database": "connected",
	})
}

// @Tags         Expenses
// @Produce      text/csv
// @Security     BearerAuth
// @Param        category  query    string false  "Filter by category"
// @Param        search    query    string false  "Search by description (case-insensitive substring match)"
// @Param        from      query    string false  "Filter from this date (YYYY-MM-DD)"
// @Param        to        query    string false  "Filter up to this date (YYYY-MM-DD)"
// @Success      200 {file} file
// @Failure      401 {object} map[string]string
// @Router       /expenses/export [get]
func (h *Handler) ExportExpenses(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	params := expense.ListParams{
		Search: r.URL.Query().Get("search"),
	}

	if catIDStr := r.URL.Query().Get("category_id"); catIDStr != "" {
		if catID, err := strconv.Atoi(catIDStr); err == nil {
			params.CategoryID = &catID
		}
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if from, err := time.Parse("2006-01-02", fromStr); err == nil {
			params.From = &from
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if to, err := time.Parse("2006-01-02", toStr); err == nil {
			to = to.Add(24*time.Hour - time.Nanosecond)
			params.To = &to
		}
	}

	expenses, err := h.store.ExportAll(userID, params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=expenses.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Amount", "Category", "Sub-Category", "Description", "Date"})

	for _, e := range expenses {
		writer.Write([]string{
			strconv.Itoa(e.ID),
			strconv.FormatFloat(e.Amount, 'f', 2, 64),
			e.CategoryName,
			e.SubCategoryName,
			e.Description,
			e.Date.Format("2006-01-02"),
		})
	}

	writer.Flush()
}
