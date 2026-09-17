package cli

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClientFollowsRedirectsWithBoundedLimit(t *testing.T) {
	client := newHTTPClient()
	require.NotNil(t, client.CheckRedirect, "the safe client must install its bounded redirect policy")

	first, err := http.NewRequest(http.MethodGet, "https://example.com/first", nil)
	require.NoError(t, err)
	second, err := http.NewRequest(http.MethodGet, "https://example.com/second", nil)
	require.NoError(t, err)

	require.NoError(t, client.CheckRedirect(second, []*http.Request{first}), "a normal redirect should be followed")

	via := make([]*http.Request, 5)
	for i := range via {
		via[i], err = http.NewRequest(http.MethodGet, "https://example.com/redirect", nil)
		require.NoError(t, err)
	}
	third, err := http.NewRequest(http.MethodGet, "https://example.com/third", nil)
	require.NoError(t, err)
	require.Error(t, client.CheckRedirect(third, via), "the redirect limit must remain enforced")
}

func TestParseDateFilter(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		got, err := parseDateFilter("")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("valid date", func(t *testing.T) {
		got, err := parseDateFilter("2024-01-15")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), *got)
	})

	t.Run("invalid format rejected", func(t *testing.T) {
		_, err := parseDateFilter("01/15/2024")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected YYYY-MM-DD")
	})

	t.Run("invalid month rejected", func(t *testing.T) {
		_, err := parseDateFilter("2024-13-01")
		require.Error(t, err)
	})
}

func TestParseDateRange(t *testing.T) {
	t.Run("both empty returns nils", func(t *testing.T) {
		since, before, err := parseDateRange("", "")
		require.NoError(t, err)
		assert.Nil(t, since)
		assert.Nil(t, before)
	})

	t.Run("only since", func(t *testing.T) {
		since, before, err := parseDateRange("2024-01-15", "")
		require.NoError(t, err)
		require.NotNil(t, since)
		assert.Nil(t, before)
		assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), *since)
	})

	t.Run("only before", func(t *testing.T) {
		since, before, err := parseDateRange("", "2024-02-01")
		require.NoError(t, err)
		assert.Nil(t, since)
		require.NotNil(t, before)
		assert.Equal(t, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), *before)
	})

	t.Run("since equals before is allowed", func(t *testing.T) {
		since, before, err := parseDateRange("2024-01-15", "2024-01-15")
		require.NoError(t, err)
		require.NotNil(t, since)
		require.NotNil(t, before)
	})

	t.Run("since before before is allowed", func(t *testing.T) {
		_, _, err := parseDateRange("2024-01-01", "2024-02-01")
		require.NoError(t, err)
	})

	t.Run("since after before is rejected", func(t *testing.T) {
		_, _, err := parseDateRange("2024-02-01", "2024-01-01")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--since")
		assert.Contains(t, err.Error(), "--before")
	})

	t.Run("invalid since surfaces error", func(t *testing.T) {
		_, _, err := parseDateRange("bogus", "2024-01-01")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected YYYY-MM-DD")
	})

	t.Run("invalid before surfaces error", func(t *testing.T) {
		_, _, err := parseDateRange("2024-01-01", "bogus")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected YYYY-MM-DD")
	})
}
