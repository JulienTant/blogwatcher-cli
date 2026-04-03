package controller

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/JulienTant/blogwatcher-cli/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBlogAndRemoveBlog(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(ctx, db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")

	_, err = AddBlog(ctx, db, "Test", "https://other.com", "", "")
	require.Error(t, err, "expected duplicate name error")

	_, err = AddBlog(ctx, db, "Other", "https://example.com", "", "")
	require.Error(t, err, "expected duplicate url error")

	err = RemoveBlog(ctx, db, blog.Name)
	require.NoError(t, err, "remove blog")
}

func TestArticleReadUnread(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(ctx, db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")
	article, err := db.AddArticle(ctx, model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	require.NoError(t, err, "add article")

	read, err := MarkArticleRead(ctx, db, article.ID)
	require.NoError(t, err, "mark read")
	require.False(t, read.IsRead, "expected original state unread")

	unread, err := MarkArticleUnread(ctx, db, article.ID)
	require.NoError(t, err, "mark unread")
	require.True(t, unread.IsRead, "expected original state read")
}

func TestGetArticlesFilters(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(ctx, db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")
	_, err = db.AddArticle(ctx, model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	require.NoError(t, err, "add article")

	articles, blogNames, err := GetArticles(ctx, db, false, "")
	require.NoError(t, err, "get articles")
	require.Len(t, articles, 1)
	require.Equal(t, blog.Name, blogNames[blog.ID])

	_, _, err = GetArticles(ctx, db, false, "Missing")
	require.Error(t, err, "expected blog not found error")
}

func TestImportOPML(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Subscriptions</title></head>
    <body>
        <outline text="Tech" title="Tech">
            <outline type="rss" text="Blog A" title="Blog A" xmlUrl="http://a.com/feed" htmlUrl="http://a.com"/>
            <outline type="rss" text="Blog B" title="Blog B" xmlUrl="http://b.com/rss" htmlUrl="http://b.com"/>
        </outline>
    </body>
</opml>`

	added, skipped, err := ImportOPML(ctx, db, strings.NewReader(opmlData))
	require.NoError(t, err)
	assert.Equal(t, 2, added)
	assert.Equal(t, 0, skipped)

	// Verify blogs were actually persisted.
	blogs, err := db.ListBlogs(ctx)
	require.NoError(t, err)
	assert.Len(t, blogs, 2)
}

func TestImportOPMLSkipsDuplicates(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	// Pre-add a blog that will conflict.
	_, err := AddBlog(ctx, db, "Blog A", "http://a.com", "http://a.com/feed", "")
	require.NoError(t, err)

	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Subscriptions</title></head>
    <body>
        <outline type="rss" text="Blog A" title="Blog A" xmlUrl="http://a.com/feed" htmlUrl="http://a.com"/>
        <outline type="rss" text="Blog B" title="Blog B" xmlUrl="http://b.com/rss" htmlUrl="http://b.com"/>
    </body>
</opml>`

	added, skipped, err := ImportOPML(ctx, db, strings.NewReader(opmlData))
	require.NoError(t, err)
	assert.Equal(t, 1, added)
	assert.Equal(t, 1, skipped)
}

func TestImportOPMLFallbackSiteURL(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	// Feed with no htmlUrl -- siteURL should fall back to feedURL.
	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Test</title></head>
    <body>
        <outline type="rss" text="NoSite" title="NoSite" xmlUrl="http://nosite.com/feed"/>
    </body>
</opml>`

	added, skipped, err := ImportOPML(ctx, db, strings.NewReader(opmlData))
	require.NoError(t, err)
	assert.Equal(t, 1, added)
	assert.Equal(t, 0, skipped)

	blog, err := db.GetBlogByName(ctx, "NoSite")
	require.NoError(t, err)
	require.NotNil(t, blog)
	assert.Equal(t, "http://nosite.com/feed", blog.URL, "site URL should fall back to feed URL")
}

func TestImportOPMLInvalidXML(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	_, _, err := ImportOPML(ctx, db, strings.NewReader("not xml"))
	require.Error(t, err)
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher-cli.db")
	db, err := storage.OpenDatabase(context.Background(), path)
	require.NoError(t, err, "open database")
	return db
}
