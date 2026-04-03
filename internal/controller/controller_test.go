package controller

import (
	"path/filepath"
	"testing"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/JulienTant/blogwatcher-cli/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestAddBlogAndRemoveBlog(t *testing.T) {
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")

	_, err = AddBlog(db, "Test", "https://other.com", "", "")
	require.Error(t, err, "expected duplicate name error")

	_, err = AddBlog(db, "Other", "https://example.com", "", "")
	require.Error(t, err, "expected duplicate url error")

	err = RemoveBlog(db, blog.Name)
	require.NoError(t, err, "remove blog")
}

func TestArticleReadUnread(t *testing.T) {
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	require.NoError(t, err, "add article")

	read, err := MarkArticleRead(db, article.ID)
	require.NoError(t, err, "mark read")
	require.False(t, read.IsRead, "expected original state unread")

	unread, err := MarkArticleUnread(db, article.ID)
	require.NoError(t, err, "mark unread")
	require.True(t, unread.IsRead, "expected original state read")
}

func TestGetArticlesFilters(t *testing.T) {
	db := openTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	require.NoError(t, err, "add blog")
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	require.NoError(t, err, "add article")

	articles, blogNames, err := GetArticles(db, false, "")
	require.NoError(t, err, "get articles")
	require.Len(t, articles, 1)
	require.Equal(t, blog.Name, blogNames[blog.ID])

	_, _, err = GetArticles(db, false, "Missing")
	require.Error(t, err, "expected blog not found error")
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher-cli.db")
	db, err := storage.OpenDatabase(path)
	require.NoError(t, err, "open database")
	return db
}
