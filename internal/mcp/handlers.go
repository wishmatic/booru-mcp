package mcp

import (
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"go.uber.org/zap"
)

type handlers struct {
	log     *zap.Logger
	catalog *catalog.Service
}

func (h *handlers) parseCategory(value string) (booru.TagCategory, error) {
	if value == "" {
		return "", nil
	}

	return booru.ParseTagCategory(value)
}

func (h *handlers) parseRating(value string) (booru.Rating, error) {
	if value == "" {
		return "", nil
	}

	return booru.ParseRating(value)
}

func (h *handlers) failure(tool string, err error) error {
	h.log.Error(tool + " failed")

	return err
}
