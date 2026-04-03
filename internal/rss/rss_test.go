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

func TestDiscoverFeedURL_XMLContentType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tag/AI/feed/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=UTF-8")
		_, writeErr := w.Write([]byte(sampleFeed))
		if writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := DiscoverFeedURL(context.Background(), server.URL+"/tag/AI/feed/", 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/tag/AI/feed/", feedURL, "should return URL directly for feed content-type")
}

func TestDiscoverFeedURL_RelSelf(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, writeErr := w.Write([]byte(`<html><head><link rel="self" type="application/rss+xml" href="/my-feed.xml" /></head></html>`))
		if writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("/my-feed.xml", func(w http.ResponseWriter, r *http.Request) {
		_, writeErr := w.Write([]byte(sampleFeed))
		if writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	feedURL, err := DiscoverFeedURL(context.Background(), server.URL, 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/my-feed.xml", feedURL, "should discover feed from rel=self link")
}
