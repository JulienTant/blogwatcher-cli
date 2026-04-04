package controller

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/JulienTant/blogwatcher-cli/internal/storage"
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

	articles, blogNames, err := GetArticles(ctx, db, false, "", "")
	require.NoError(t, err, "get articles")
	require.Len(t, articles, 1)
	require.Equal(t, blog.Name, blogNames[blog.ID])

	_, _, err = GetArticles(ctx, db, false, "Missing", "")
	require.Error(t, err, "expected blog not found error")
}

func TestGetArticlesFilterByCategory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(ctx, db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")

	_, err = db.AddArticle(ctx, model.Article{BlogID: blog.ID, Title: "Go Post", URL: "https://example.com/1", Categories: []string{"Go", "Programming"}})
	require.NoError(t, err, "add article")
	_, err = db.AddArticle(ctx, model.Article{BlogID: blog.ID, Title: "Rust Post", URL: "https://example.com/2", Categories: []string{"Rust"}})
	require.NoError(t, err, "add article")

	// Filter by Go
	articles, _, err := GetArticles(ctx, db, false, "", "Go")
	require.NoError(t, err, "get articles by category")
	require.Len(t, articles, 1)
	require.Equal(t, "Go Post", articles[0].Title)

	// No filter returns all
	all, _, err := GetArticles(ctx, db, false, "", "")
	require.NoError(t, err, "get all articles")
	require.Len(t, all, 2)
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher-cli.db")
	db, err := storage.OpenDatabase(context.Background(), path)
	require.NoError(t, err, "open database")
	return db
}
