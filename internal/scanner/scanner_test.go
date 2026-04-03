package scanner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/JulienTant/blogwatcher-cli/internal/storage"
	"github.com/stretchr/testify/require"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
<title>Example Feed</title>
<item>
<title>First</title>
<link>https://example.com/1</link>
</item>
<item>
<title>Second</title>
<link>https://example.com/2</link>
</item>
</channel>
</rss>`

func TestScanBlogRSS(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(sampleFeed)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(ctx, model.Blog{Name: "Test", URL: "https://example.com", FeedURL: server.URL})
	require.NoError(t, err, "add blog")

	result := ScanBlog(ctx, db, blog)
	require.Equal(t, 2, result.NewArticles)
	require.Equal(t, "rss", result.Source)

	articles, err := db.ListArticles(ctx, false, nil)
	require.NoError(t, err, "list articles")
	require.Len(t, articles, 2)
}

func TestScanBlogScraperFallback(t *testing.T) {
	ctx := context.Background()
	html := `<!DOCTYPE html>
<html>
<body>
  <article><h2><a href="/one">First</a></h2></article>
</body>
</html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(ctx, model.Blog{Name: "Test", URL: server.URL, FeedURL: server.URL + "/feed.xml", ScrapeSelector: "article h2 a"})
	require.NoError(t, err, "add blog")

	result := ScanBlog(ctx, db, blog)
	require.Equal(t, "scraper", result.Source)
	require.Equal(t, 1, result.NewArticles)
	require.Empty(t, result.Error)
}

func TestScanAllBlogsConcurrent(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(sampleFeed)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	for i, name := range []string{"TestA", "TestB"} {
		_, err := db.AddBlog(ctx, model.Blog{Name: name, URL: "https://example.com/" + name, FeedURL: server.URL})
		require.NoError(t, err, "add blog %d", i)
	}

	results, err := ScanAllBlogs(ctx, db, 2)
	require.NoError(t, err, "scan all blogs")
	require.Len(t, results, 2)
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher-cli.db")
	db, err := storage.OpenDatabase(context.Background(), path)
	require.NoError(t, err, "open database")
	return db
}

func TestScanBlogRespectsExistingArticles(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(sampleFeed)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(ctx, model.Blog{Name: "Test", URL: "https://example.com", FeedURL: server.URL})
	require.NoError(t, err, "add blog")

	_, err = db.AddArticle(ctx, model.Article{BlogID: blog.ID, Title: "First", URL: "https://example.com/1", DiscoveredDate: ptrTime(time.Now())})
	require.NoError(t, err, "add article")

	result := ScanBlog(ctx, db, blog)
	require.Equal(t, 1, result.NewArticles)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
