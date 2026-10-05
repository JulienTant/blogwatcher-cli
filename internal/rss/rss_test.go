package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
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

	feedURL, err := newTestFetcher().DiscoverFeedURL(context.Background(), server.URL+"/tag/AI/feed/")
	require.NoError(t, err)
	require.Equal(t, server.URL+"/tag/AI/feed/", feedURL, "should return URL directly for feed content-type")
}

func TestDiscoverFeedURL_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newTestFetcher().DiscoverFeedURL(context.Background(), server.URL)
	require.Error(t, err, "should return error for 5xx")
	require.Contains(t, err.Error(), "server error status 503")
	require.True(t, IsFeedError(err), "should be a FeedParseError so it's retryable")
}

func TestDiscoverFeedURL_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	feedURL, err := newTestFetcher().DiscoverFeedURL(context.Background(), server.URL)
	require.NoError(t, err, "404 should not be an error")
	require.Empty(t, feedURL, "should return empty for 404")
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

	feedURL, err := newTestFetcher().DiscoverFeedURL(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, server.URL+"/my-feed.xml", feedURL, "should discover feed from rel=self link")
}

// podcastFeed mirrors the shape of a real podcast feed: no <link> on any item,
// an opaque non-permalink <guid> per episode, and the episode file only in
// <enclosure url>. One item carries a permalink <guid> instead, and one carries
// no usable URL at all. The body is fabricated — no real feed is pasted here.
const podcastFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Podcast Example</title>
<item>
<title>Link wins</title>
<link>https://example.com/episodes/1</link>
<guid isPermaLink="false">opaque-1</guid>
<enclosure url="https://cdn.example.com/1.mp3" length="1" type="audio/mpeg" />
</item>
<item>
<title>Permalink guid</title>
<guid>https://example.com/episodes/2</guid>
</item>
<item>
<title>Enclosure fallback</title>
<guid isPermaLink="false">opaque-3</guid>
<enclosure url="https://cdn.example.com/3.mp3" length="1" type="audio/mpeg" />
</item>
<item>
<title>Nothing to link</title>
<guid isPermaLink="false">opaque-4</guid>
</item>
</channel>
</rss>`

func TestParseFeedPodcastURLFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(podcastFeed)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	articles, err := newTestFetcher().ParseFeed(context.Background(), server.URL)
	require.NoError(t, err, "parse feed")
	require.Len(t, articles, 3, "the item with no link, no permalink guid and no enclosure is dropped")

	require.Equal(t, "Link wins", articles[0].Title)
	require.Equal(t, "https://example.com/episodes/1", articles[0].URL, "<link> is unchanged when present")

	require.Equal(t, "Permalink guid", articles[1].Title)
	require.Equal(t, "https://example.com/episodes/2", articles[1].URL, "a permalink <guid> is the first fallback")

	require.Equal(t, "Enclosure fallback", articles[2].Title)
	require.Equal(t, "https://cdn.example.com/3.mp3", articles[2].URL, "<enclosure url> is the last fallback")

	for _, article := range articles {
		require.NotEqual(t, "opaque-1", article.URL, "a non-permalink guid is never used as a URL")
		require.NotEqual(t, "opaque-3", article.URL, "a non-permalink guid is never used as a URL")
	}
}

func TestItemURL(t *testing.T) {
	cases := []struct {
		name     string
		item     *gofeed.Item
		expected string
	}{
		{
			name:     "link present",
			item:     &gofeed.Item{Link: "https://example.com/a"},
			expected: "https://example.com/a",
		},
		{
			name:     "permalink guid",
			item:     &gofeed.Item{GUID: "https://example.com/b"},
			expected: "https://example.com/b",
		},
		{
			name:     "non-permalink guid and enclosure",
			item:     &gofeed.Item{GUID: "opaque-c", Enclosures: []*gofeed.Enclosure{{URL: "https://cdn.example.com/c.mp3"}}},
			expected: "https://cdn.example.com/c.mp3",
		},
		{
			name:     "no URL at all",
			item:     &gofeed.Item{GUID: "opaque-d"},
			expected: "",
		},
		{
			name:     "empty enclosure URL is skipped",
			item:     &gofeed.Item{GUID: "opaque-e", Enclosures: []*gofeed.Enclosure{{URL: "  "}, {URL: "https://cdn.example.com/e.mp3"}}},
			expected: "https://cdn.example.com/e.mp3",
		},
		{
			name:     "relative guid is not a permalink",
			item:     &gofeed.Item{GUID: "/episodes/f"},
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, itemURL(tc.item))
		})
	}
}
