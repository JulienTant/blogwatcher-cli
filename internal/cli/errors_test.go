package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/JulienTant/blogwatcher-cli/internal/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkErrorReturnsNilForNilInput(t *testing.T) {
	require.NoError(t, markError(nil))
}

func TestIsPrintedReturnsTrueForMarkedError(t *testing.T) {
	err := markError(errors.New("something"))
	assert.True(t, isPrinted(err))
}

func TestIsPrintedReturnsFalseForUnmarkedError(t *testing.T) {
	err := errors.New("not printed")
	assert.False(t, isPrinted(err))
}

func TestPrintedErrorUnwrapWithErrorsIs(t *testing.T) {
	sentinel := errors.New("sentinel")
	wrapped := markError(sentinel)

	assert.ErrorIs(t, wrapped, sentinel)
}

func TestPrintedErrorUnwrapWithBlogNotFoundError(t *testing.T) {
	original := controller.BlogNotFoundError{Name: "myblog"}
	wrapped := markError(original)

	var target controller.BlogNotFoundError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "myblog", target.Name)
}

func TestPrintedErrorUnwrapWithBlogAlreadyExistsError(t *testing.T) {
	original := controller.BlogAlreadyExistsError{Field: "name", Value: "dup"}
	wrapped := markError(original)

	var target controller.BlogAlreadyExistsError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "name", target.Field)
	assert.Equal(t, "dup", target.Value)
}

func TestPrintedErrorUnwrapWithArticleNotFoundError(t *testing.T) {
	original := controller.ArticleNotFoundError{ID: 42}
	wrapped := markError(original)

	var target controller.ArticleNotFoundError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, int64(42), target.ID)
}

func TestPrintedErrorUnwrapDoubleWrapped(t *testing.T) {
	original := controller.BlogNotFoundError{Name: "deep"}
	fmtWrapped := fmt.Errorf("wrapper: %w", original)
	printed := markError(fmtWrapped)

	var target controller.BlogNotFoundError
	require.ErrorAs(t, printed, &target)
	assert.Equal(t, "deep", target.Name)
}
