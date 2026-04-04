package controller

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlogNotFoundErrorMessage(t *testing.T) {
	err := BlogNotFoundError{Name: "testblog"}
	assert.Equal(t, "Blog 'testblog' not found", err.Error())
}

func TestBlogAlreadyExistsErrorMessage(t *testing.T) {
	err := BlogAlreadyExistsError{Field: "URL", Value: "https://example.com"}
	assert.Equal(t, "Blog with URL 'https://example.com' already exists", err.Error())
}

func TestArticleNotFoundErrorMessage(t *testing.T) {
	err := ArticleNotFoundError{ID: 99}
	assert.Equal(t, "Article 99 not found", err.Error())
}

func TestBlogNotFoundErrorWrappedWithFmtErrorf(t *testing.T) {
	original := BlogNotFoundError{Name: "wrapped"}
	wrapped := fmt.Errorf("operation failed: %w", original)

	var target BlogNotFoundError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "wrapped", target.Name)
}

func TestBlogAlreadyExistsErrorWrappedWithFmtErrorf(t *testing.T) {
	original := BlogAlreadyExistsError{Field: "name", Value: "dup"}
	wrapped := fmt.Errorf("add failed: %w", original)

	var target BlogAlreadyExistsError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "name", target.Field)
	assert.Equal(t, "dup", target.Value)
}

func TestArticleNotFoundErrorWrappedWithFmtErrorf(t *testing.T) {
	original := ArticleNotFoundError{ID: 7}
	wrapped := fmt.Errorf("mark read failed: %w", original)

	var target ArticleNotFoundError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, int64(7), target.ID)
}
