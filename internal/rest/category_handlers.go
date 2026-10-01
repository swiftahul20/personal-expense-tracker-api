package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/swiftahul20/expense-tracker/internal/auth"
	"github.com/swiftahul20/expense-tracker/internal/category"
)

type CategoryHandler struct {
	store category.Store
}

func NewCategoryHandler(store category.Store) *CategoryHandler {
	return &CategoryHandler{store: store}
}

type categoryRequest struct {
	Name string `json:"name"`
}

// ListCategories godoc
// @Summary      List categories
// @Description  Returns the authenticated user's categories with nested sub-categories
// @Tags         categories
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} category.Category
// @Failure      401 {object} map[string]string
// @Router       /categories [get]
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	categories, err := h.store.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, categories)
}

// CreateCategory godoc
// @Summary      Create a category
// @Tags         categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        category body categoryRequest true "Category name"
// @Success      201 {object} category.Category
// @Failure      400 {object} map[string]string
// @Router       /categories [post]
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.store.Create(userID, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// UpdateCategory godoc
// @Summary      Rename a category
// @Tags         categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Category ID"
// @Param        category body categoryRequest true "New name"
// @Success      200 {object} category.Category
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /categories/{id} [put]
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.store.Update(userID, id, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteCategory godoc
// @Summary      Delete a category
// @Description  Existing expenses referencing this category will have their category set to null
// @Tags         categories
// @Security     BearerAuth
// @Param        id path int true "Category ID"
// @Success      204 "No Content"
// @Failure      404 {object} map[string]string
// @Router       /categories/{id} [delete]
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
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

// CreateSubCategory godoc
// @Summary      Create a sub-category under a category
// @Tags         categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Parent category ID"
// @Param        subcategory body categoryRequest true "Sub-category name"
// @Success      201 {object} category.SubCategory
// @Failure      400 {object} map[string]string
// @Router       /categories/{id}/sub-categories [post]
func (h *CategoryHandler) CreateSubCategory(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	categoryID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.store.CreateSubCategory(userID, categoryID, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// UpdateSubCategory godoc
// @Summary      Rename a sub-category
// @Tags         categories
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Sub-category ID"
// @Param        subcategory body categoryRequest true "New name"
// @Success      200 {object} category.SubCategory
// @Failure      400 {object} map[string]string
// @Router       /sub-categories/{id} [put]
func (h *CategoryHandler) UpdateSubCategory(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.store.UpdateSubCategory(userID, id, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteSubCategory godoc
// @Summary      Delete a sub-category
// @Tags         categories
// @Security     BearerAuth
// @Param        id path int true "Sub-category ID"
// @Success      204 "No Content"
// @Failure      404 {object} map[string]string
// @Router       /sub-categories/{id} [delete]
func (h *CategoryHandler) DeleteSubCategory(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.store.DeleteSubCategory(userID, id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
