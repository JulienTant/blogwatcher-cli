package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/stretchr/testify/require"
)

func TestDatabaseCreatesFileAndCRUD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	_, err = os.Stat(path)
	require.NoError(t, err, "expected db file to exist")

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err, "add blog")
	require.NotEqual(t, int64(0), blog.ID, "expected blog ID")

	fetched, err := db.GetBlog(blog.ID)
	require.NoError(t, err, "get blog")
	require.NotNil(t, fetched)
	require.Equal(t, "Test", fetched.Name)

	articles := []model.Article{
		{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"},
		{BlogID: blog.ID, Title: "Two", URL: "https://example.com/2"},
	}
	count, err := db.AddArticlesBulk(articles)
	require.NoError(t, err, "add articles bulk")
	require.Equal(t, 2, count)

	list, err := db.ListArticles(false, nil)
	require.NoError(t, err, "list articles")
	require.Len(t, list, 2)

	ok, err := db.MarkArticleRead(list[0].ID)
	require.NoError(t, err, "mark read")
	require.True(t, ok)

	updated, err := db.GetArticle(list[0].ID)
	require.NoError(t, err, "get article")
	require.NotNil(t, updated)
	require.True(t, updated.IsRead)

	now := time.Now()
	err = db.UpdateBlogLastScanned(blog.ID, now)
	require.NoError(t, err, "update last scanned")

	deleted, err := db.RemoveBlog(blog.ID)
	require.NoError(t, err, "remove blog")
	require.True(t, deleted)
}

func TestGetExistingArticleURLs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err, "add blog")

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"})
	require.NoError(t, err, "add article")

	existing, err := db.GetExistingArticleURLs([]string{"https://example.com/1", "https://example.com/2"})
	require.NoError(t, err, "get existing")
	_, ok := existing["https://example.com/1"]
	require.True(t, ok, "expected existing url")
	_, ok = existing["https://example.com/2"]
	require.False(t, ok, "did not expect url")
}

func TestDatabaseForeignKeyEnforced(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	_, err = db.AddArticle(model.Article{BlogID: 9999, Title: "Orphan", URL: "https://example.com/orphan"})
	require.Error(t, err, "expected foreign key error for missing blog")
}

func TestBlogOptionalFieldsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err, "add blog")

	fetched, err := db.GetBlog(blog.ID)
	require.NoError(t, err, "get blog")
	require.NotNil(t, fetched)
	require.Equal(t, "", fetched.FeedURL)
	require.Equal(t, "", fetched.ScrapeSelector)
}

func TestBlogTimeRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	now := time.Date(2025, 1, 2, 3, 4, 5, 6, time.UTC)
	blog, err := db.AddBlog(model.Blog{
		Name:        "Test",
		URL:         "https://example.com",
		LastScanned: &now,
	})
	require.NoError(t, err, "add blog")

	fetched, err := db.GetBlog(blog.ID)
	require.NoError(t, err, "get blog")
	require.NotNil(t, fetched)
	require.NotNil(t, fetched.LastScanned)
	require.True(t, fetched.LastScanned.Equal(now), "expected last scanned %s, got %s", now.Format(time.RFC3339Nano), fetched.LastScanned.Format(time.RFC3339Nano))
}

func TestArticleTimeRoundTripAndNilDiscoveredDate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err, "add blog")

	published := time.Date(2024, 12, 31, 23, 59, 59, 123, time.UTC)
	article, err := db.AddArticle(model.Article{
		BlogID:        blog.ID,
		Title:         "Title",
		URL:           "https://example.com/1",
		PublishedDate: &published,
	})
	require.NoError(t, err, "add article")

	fetched, err := db.GetArticle(article.ID)
	require.NoError(t, err, "get article")
	require.NotNil(t, fetched)
	require.NotNil(t, fetched.PublishedDate)
	require.True(t, fetched.PublishedDate.Equal(published), "expected published date %s, got %s", published.Format(time.RFC3339Nano), fetched.PublishedDate.Format(time.RFC3339Nano))
	require.Nil(t, fetched.DiscoveredDate, "expected discovered date nil when not set")
}

func TestListArticlesFiltersAndOrdering(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	blogA, err := db.AddBlog(model.Blog{Name: "A", URL: "https://a.example.com"})
	require.NoError(t, err, "add blog")
	blogB, err := db.AddBlog(model.Blog{Name: "B", URL: "https://b.example.com"})
	require.NoError(t, err, "add blog")

	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	first, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "Old", URL: "https://a.example.com/old", DiscoveredDate: &t1})
	require.NoError(t, err, "add article")
	second, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "New", URL: "https://a.example.com/new", DiscoveredDate: &t2})
	require.NoError(t, err, "add article")
	_, err = db.AddArticle(model.Article{BlogID: blogB.ID, Title: "Other", URL: "https://b.example.com/1", DiscoveredDate: &t2})
	require.NoError(t, err, "add article")

	_, err = db.MarkArticleRead(first.ID)
	require.NoError(t, err, "mark read")

	all, err := db.ListArticles(false, nil)
	require.NoError(t, err, "list articles")
	require.Len(t, all, 3)
	require.Equal(t, second.ID, all[0].ID, "expected newest article first")

	unread, err := db.ListArticles(true, nil)
	require.NoError(t, err, "list unread")
	require.Len(t, unread, 2)

	blogID := blogB.ID
	filtered, err := db.ListArticles(false, &blogID)
	require.NoError(t, err, "list by blog")
	require.Len(t, filtered, 1)
	require.Equal(t, blogB.ID, filtered[0].BlogID)
}

func TestBulkInsertDuplicateRollbackAndEmpty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err, "add blog")

	count, err := db.AddArticlesBulk(nil)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Existing", URL: "https://example.com/existing"})
	require.NoError(t, err, "add article")

	dupArticles := []model.Article{
		{BlogID: blog.ID, Title: "Dup", URL: "https://example.com/dup"},
		{BlogID: blog.ID, Title: "Dup2", URL: "https://example.com/dup"},
	}
	_, err = db.AddArticlesBulk(dupArticles)
	require.Error(t, err, "expected bulk insert to fail on duplicate url")

	articles, err := db.ListArticles(false, nil)
	require.NoError(t, err, "list articles")
	require.Len(t, articles, 1, "expected rollback on duplicate")
}

func TestLookupHelpers(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err, "open database")
	defer func() { require.NoError(t, db.Close()) }()

	blogByName, err := db.GetBlogByName("missing")
	require.NoError(t, err)
	require.Nil(t, blogByName)

	blogByURL, err := db.GetBlogByURL("https://missing.example.com")
	require.NoError(t, err)
	require.Nil(t, blogByURL)

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err, "add blog")
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	require.NoError(t, err, "add article")

	found, err := db.GetArticleByURL(article.URL)
	require.NoError(t, err)
	require.NotNil(t, found)

	exists, err := db.ArticleExists(article.URL)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = db.ArticleExists("https://example.com/missing")
	require.NoError(t, err)
	require.False(t, exists)
}
