package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Example Feed</title>
<item>
<title>First</title>
<link>https://example.com/1</link>
<pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
</item>
<item>
<title>Second</title>
<link>https://example.com/2</link>
</item>
</channel>
</rss>`

func newTestFetcher() *Fetcher {
	return NewFetcher(&http.Client{Timeout: 2 * time.Second})
}

func TestParseFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(sampleFeed)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	articles, err := newTestFetcher().ParseFeed(context.Background(), server.URL)
	require.NoError(t, err, "parse feed")
	require.Len(t, articles, 2)
	require.NotNil(t, articles[0].PublishedDate)
}

func TestParseFeedWithCategories(t *testing.T) {
	feedWithCategories := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Example Feed</title>
<item>
<title>Tagged Post</title>
<link>https://example.com/tagged</link>
<category>AI</category>
<category>Machine Learning</category>
</item>
<item>
<title>Plain Post</title>
<link>https://example.com/plain</link>
</item>
</channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(feedWithCategories)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	articles, err := newTestFetcher().ParseFeed(context.Background(), server.URL)
	require.NoError(t, err, "parse feed")
	require.Len(t, articles, 2)

	require.Equal(t, []string{"AI", "Machine Learning"}, articles[0].Categories)
	require.Nil(t, articles[1].Categories)
}

func TestDiscoverFeedURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(`<html><head><link rel="alternate" type="application/rss+xml" href="/feed.xml" /></head></html>`)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(sampleFeed)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := newTestFetcher().DiscoverFeedURL(context.Background(), server.URL)
	require.NoError(t, err, "discover feed")
	require.NotEmpty(t, feedURL, "expected feed url")
}
