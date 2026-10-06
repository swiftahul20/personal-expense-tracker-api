package rest

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/swiftahul20/expense-tracker/internal/auth"
	"github.com/swiftahul20/expense-tracker/internal/category"
	"github.com/swiftahul20/expense-tracker/internal/receipt"
)

type ReceiptHandler struct {
	categoryStore category.Store
	scanner       *receipt.Client
}

func NewReceiptHandler(categoryStore category.Store, scanner *receipt.Client) *ReceiptHandler {
	return &ReceiptHandler{categoryStore: categoryStore, scanner: scanner}
}

func (h *ReceiptHandler) ScanReceipt(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form upload")
		return
	}

	file, header, err := r.FormFile("receipt")
	if err != nil {
		file, header, err = r.FormFile("image")
		if err != nil {
			writeError(w, http.StatusBadRequest, "receipt image is required")
			return
		}
	}
	defer file.Close()

	if header == nil || header.Size == 0 {
		writeError(w, http.StatusBadRequest, "receipt image is required")
		return
	}

	image, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read receipt image")
		return
	}
	if len(image) == 0 {
		writeError(w, http.StatusBadRequest, "receipt image is empty")
		return
	}

	mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(image)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		writeError(w, http.StatusBadRequest, "uploaded file is not a valid image")
		return
	}

	categories, err := h.categoryStore.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "receipt scanning is not configured")
		return
	}

	scanResult, err := h.scanner.ScanImage(r.Context(), image, mimeType, categories)
	if err != nil {
		if errors.Is(err, receipt.ErrNoAPIKey) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		if errors.Is(err, receipt.ErrNotReceipt) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, receipt.ErrInvalidImage) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, scanResult)
}
