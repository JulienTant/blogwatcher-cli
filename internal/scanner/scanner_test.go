package scanner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/JulienTant/blogwatcher-cli/internal/rss"
	"github.com/JulienTant/blogwatcher-cli/internal/scraper"
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

func newTestScanner() *Scanner {
	client := &http.Client{Timeout: 2 * time.Second}
	return NewScanner(rss.NewFetcher(client), scraper.NewScraper(client))
}

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

	result, scanErr := newTestScanner().ScanBlog(ctx, db, blog)
	require.NoError(t, scanErr)
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

	result, scanErr := newTestScanner().ScanBlog(ctx, db, blog)
	require.NoError(t, scanErr)
	require.Equal(t, "scraper", result.Source)
	require.Equal(t, 1, result.NewArticles)
}

func TestScanAllBlogsConcurrent(t *testing.T) {
	ctx := context.Background()

	feedTemplate := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0"><channel><title>%s</title>
<item><title>Post 1</title><link>https://%s.example.com/1</link></item>
<item><title>Post 2</title><link>https://%s.example.com/2</link></item>
</channel></rss>`

	mux := http.NewServeMux()
	for _, name := range []string{"a", "b"} {
		feed := fmt.Sprintf(feedTemplate, name, name, name)
		mux.HandleFunc("/"+name+"/feed", func(w http.ResponseWriter, r *http.Request) {
			if _, writeErr := w.Write([]byte(feed)); writeErr != nil {
				http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			}
		})
	}
	server := httptest.NewServer(mux)
	defer server.Close()

	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	for _, name := range []string{"a", "b"} {
		_, err := db.AddBlog(ctx, model.Blog{
			Name:    "Test-" + name,
			URL:     "https://" + name + ".example.com",
			FeedURL: server.URL + "/" + name + "/feed",
		})
		require.NoError(t, err, "add blog %s", name)
	}

	results, err := newTestScanner().ScanAllBlogs(ctx, db, 2)
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

	result, scanErr := newTestScanner().ScanBlog(ctx, db, blog)
	require.NoError(t, scanErr)
	require.Equal(t, 1, result.NewArticles)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
