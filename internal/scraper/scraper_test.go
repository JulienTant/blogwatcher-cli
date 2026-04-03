package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScrapeBlog(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
  <article><h2><a href="/one">First</a></h2></article>
  <article><h2><a href="/one">First Duplicate</a></h2></article>
  <div class="post"><h3><span><a href="/two" title="Second">Ignore Text</a></span></h3></div>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	articles, err := ScrapeBlog(server.URL, "article h2 a, .post", 2*time.Second)
	require.NoError(t, err, "scrape blog")
	require.Len(t, articles, 2)
	require.NotEqual(t, "", articles[0].URL)
	require.NotEqual(t, "", articles[1].URL)
}
