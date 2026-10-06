package receipt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/swiftahul20/expense-tracker/internal/category"
)

var (
	ErrInvalidImage  = errors.New("uploaded file is not a valid image")
	ErrNotReceipt    = errors.New("the uploaded image does not appear to be a receipt")
	ErrInvalidOutput = errors.New("LLM response could not be parsed as structured receipt data")
	ErrNoAPIKey      = errors.New("LLM API key is not configured")
)

type ScanResult struct {
	Amount          float64 `json:"amount"`
	CategoryID      *int    `json:"category_id"`
	CategoryName    *string `json:"category_name"`
	SubCategoryID   *int    `json:"sub_category_id"`
	SubCategoryName *string `json:"sub_category_name"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	ConfidenceNote  *string `json:"confidence_note,omitempty"`
}

func (r ScanResult) Validate() error {
	if r.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if strings.TrimSpace(r.Description) == "" {
		return errors.New("description is required")
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return fmt.Errorf("date must be a valid YYYY-MM-DD value: %w", err)
	}
	return nil
}

func BuildPrompt(categories []category.Category) string {
	var b strings.Builder
	b.WriteString("You extract receipt data from a single uploaded image. Return only valid JSON with no markdown fences. ")
	b.WriteString("If the image is not a receipt, return {\"error\":\"not_a_receipt\"}. ")
	b.WriteString("Extract: amount, description, date, category_name, sub_category_name, confidence_note. ")
	b.WriteString("The date must be YYYY-MM-DD. amount must be a positive number. description should be concise and merchant/product oriented.\n")
	b.WriteString("Match the receipt to the user's existing categories/sub-categories. If no category fits well, set category_name and sub_category_name to null.\n")
	b.WriteString("Available categories:\n")
	if len(categories) == 0 {
		b.WriteString("- No categories configured for this user. If no category matches, return null values.\n")
		return b.String()
	}
	for _, c := range categories {
		b.WriteString("- ")
		b.WriteString(c.Name)
		if len(c.SubCategories) > 0 {
			b.WriteString(" (sub-categories: ")
			for i, sc := range c.SubCategories {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(sc.Name)
			}
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	b.WriteString("Return JSON in this shape: {\"amount\": 12345, \"description\": \"Example restaurant\", \"date\": \"2026-09-27\", \"category_name\": \"Food\", \"sub_category_name\": \"Dine-in\", \"confidence_note\": \"optional note\"}\n")
	return b.String()
}

func ParseLLMResponse(raw string, categories []category.Category) (ScanResult, error) {
	payload, err := extractJSONPayload(raw)
	if err != nil {
		return ScanResult{}, err
	}

	if errorValue, ok := payload["error"]; ok && strings.EqualFold(strings.TrimSpace(fmt.Sprint(errorValue)), "not_a_receipt") {
		return ScanResult{}, ErrNotReceipt
	}

	amount, err := parseAmount(payload["amount"])
	if err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}

	result := ScanResult{
		Amount:      amount,
		Description: normalizeString(payload["description"]),
		Date:        normalizeString(payload["date"]),
	}

	if confidence, ok := payload["confidence_note"]; ok && normalizeString(confidence) != "" {
		value := normalizeString(confidence)
		result.ConfidenceNote = &value
	}

	categoryName := normalizeString(payload["category_name"])
	subCategoryName := normalizeString(payload["sub_category_name"])
	result.CategoryID, result.CategoryName, result.SubCategoryID, result.SubCategoryName = resolveCategory(categories, categoryName, subCategoryName)

	if result.Description == "" {
		return ScanResult{}, fmt.Errorf("%w: description is required", ErrInvalidOutput)
	}
	if result.Date == "" {
		return ScanResult{}, fmt.Errorf("%w: date is required", ErrInvalidOutput)
	}
	if err := result.Validate(); err != nil {
		return ScanResult{}, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	return result, nil
}

func extractJSONPayload(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrInvalidOutput
	}

	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "```json"))
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "```"))
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, ErrInvalidOutput
	}
	trimmed = trimmed[start : end+1]

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	return payload, nil
}

func parseAmount(value any) (float64, error) {
	switch v := value.(type) {
	case nil:
		return 0, errors.New("amount is required")
	case float64:
		return v, validateAmount(v)
	case float32:
		return float64(v), validateAmount(float64(v))
	case int:
		return float64(v), validateAmount(float64(v))
	case int64:
		return float64(v), validateAmount(float64(v))
	case int32:
		return float64(v), validateAmount(float64(v))
	case string:
		amount, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, err
		}
		return amount, validateAmount(amount)
	default:
		return 0, errors.New("amount is not numeric")
	}
}

func validateAmount(amount float64) error {
	if amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	return nil
}

func normalizeString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func resolveCategory(categories []category.Category, categoryName, subCategoryName string) (*int, *string, *int, *string) {
	categoryName = strings.TrimSpace(categoryName)
	subCategoryName = strings.TrimSpace(subCategoryName)
	if categoryName == "" && subCategoryName == "" {
		return nil, nil, nil, nil
	}

	for _, c := range categories {
		if categoryName != "" && !strings.EqualFold(c.Name, categoryName) {
			continue
		}
		categoryMatch := c.ID
		categoryNameValue := c.Name
		if subCategoryName != "" {
			for _, sc := range c.SubCategories {
				if !strings.EqualFold(sc.Name, subCategoryName) {
					continue
				}
				categoryNamePtr := categoryNameValue
				subID := sc.ID
				subName := sc.Name
				return &categoryMatch, &categoryNamePtr, &subID, &subName
			}
		}
		if categoryName != "" {
			categoryNamePtr := categoryNameValue
			return &categoryMatch, &categoryNamePtr, nil, nil
		}
	}

	if subCategoryName != "" {
		for _, c := range categories {
			for _, sc := range c.SubCategories {
				if strings.EqualFold(sc.Name, subCategoryName) {
					categoryMatch := c.ID
					categoryNameValue := c.Name
					subMatch := sc.ID
					subNameValue := sc.Name
					return &categoryMatch, &categoryNameValue, &subMatch, &subNameValue
				}
			}
		}
	}

	if categoryName != "" {
		for _, c := range categories {
			if strings.EqualFold(c.Name, categoryName) {
				categoryMatch := c.ID
				categoryNameValue := c.Name
				return &categoryMatch, &categoryNameValue, nil, nil
			}
		}
	}

	return nil, nil, nil, nil
}
