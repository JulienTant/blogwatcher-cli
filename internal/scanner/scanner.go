package scanner

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/JulienTant/blogwatcher-cli/internal/model"
	"github.com/JulienTant/blogwatcher-cli/internal/rss"
	"github.com/JulienTant/blogwatcher-cli/internal/scraper"
	"github.com/JulienTant/blogwatcher-cli/internal/storage"
)

type ScanResult struct {
	BlogName    string
	NewArticles int
	TotalFound  int
	Source      string
}

// Scanner orchestrates blog scanning using a Fetcher and Scraper.
type Scanner struct {
	fetcher *rss.Fetcher
	scraper *scraper.Scraper
}

// NewScanner creates a Scanner with the given fetcher and scraper.
func NewScanner(fetcher *rss.Fetcher, scraper *scraper.Scraper) *Scanner {
	return &Scanner{fetcher: fetcher, scraper: scraper}
}

func (s *Scanner) ScanBlog(ctx context.Context, db *storage.Database, blog model.Blog) (ScanResult, error) {
	var (
		articles []model.Article
		source   = "none"
	)

	feedURL := blog.FeedURL
	if feedURL == "" {
		discovered, err := s.fetcher.DiscoverFeedURL(ctx, blog.URL)
		if err != nil {
			return ScanResult{BlogName: blog.Name}, err
		}
		if discovered != "" {
			feedURL = discovered
		}
	}

	if feedURL != "" {
		feedArticles, err := s.fetcher.ParseFeed(ctx, feedURL)
		if err != nil {
			// If there's a scraper fallback, try it before giving up.
			if blog.ScrapeSelector == "" {
				return ScanResult{BlogName: blog.Name}, err
			}
			// Try scraper as fallback.
			scrapedArticles, scrapeErr := s.scraper.ScrapeBlog(ctx, blog.URL, blog.ScrapeSelector)
			if scrapeErr != nil {
				return ScanResult{BlogName: blog.Name}, fmt.Errorf("RSS: %w; Scraper: %w", err, scrapeErr)
			}
			articles = convertScrapedArticles(blog.ID, scrapedArticles)
			source = "scraper"
		} else {
			articles = convertFeedArticles(blog.ID, feedArticles)
			source = "rss"
			// Persist discovered feed URL only after successful parse.
			if blog.FeedURL != feedURL {
				blog.FeedURL = feedURL
				if err := db.UpdateBlog(ctx, blog); err != nil {
					return ScanResult{BlogName: blog.Name}, err
				}
			}
		}
	} else if blog.ScrapeSelector != "" {
		scrapedArticles, err := s.scraper.ScrapeBlog(ctx, blog.URL, blog.ScrapeSelector)
		if err != nil {
			return ScanResult{BlogName: blog.Name}, err
		}
		articles = convertScrapedArticles(blog.ID, scrapedArticles)
		source = "scraper"
	}

	seenURLs := make(map[string]struct{})
	uniqueArticles := make([]model.Article, 0, len(articles))
	for _, article := range articles {
		if _, exists := seenURLs[article.URL]; exists {
			continue
		}
		seenURLs[article.URL] = struct{}{}
		uniqueArticles = append(uniqueArticles, article)
	}

	urlList := make([]string, 0, len(seenURLs))
	for url := range seenURLs {
		urlList = append(urlList, url)
	}

	existing, err := db.GetExistingArticleURLs(ctx, urlList)
	if err != nil {
		return ScanResult{BlogName: blog.Name}, err
	}

	discoveredAt := time.Now()
	newArticles := make([]model.Article, 0, len(uniqueArticles))
	for _, article := range uniqueArticles {
		if _, exists := existing[article.URL]; exists {
			continue
		}
		article.DiscoveredDate = &discoveredAt
		newArticles = append(newArticles, article)
	}

	newCount := 0
	if len(newArticles) > 0 {
		count, err := db.AddArticlesBulk(ctx, newArticles)
		if err != nil {
			return ScanResult{BlogName: blog.Name}, err
		}
		newCount = count
	}

	if err := db.UpdateBlogLastScanned(ctx, blog.ID, time.Now()); err != nil {
		return ScanResult{BlogName: blog.Name}, err
	}

	return ScanResult{
		BlogName:    blog.Name,
		NewArticles: newCount,
		TotalFound:  len(seenURLs),
		Source:      source,
	}, nil
}

func (s *Scanner) ScanAllBlogs(ctx context.Context, db *storage.Database, workers int) ([]ScanResult, error) {
	blogs, err := db.ListBlogs(ctx)
	if err != nil {
		return nil, err
	}
	if workers <= 1 {
		results := make([]ScanResult, 0, len(blogs))
		for _, blog := range blogs {
			result, err := s.ScanBlog(ctx, db, blog)
			if err != nil {
				return nil, fmt.Errorf("scan %s: %w", blog.Name, err)
			}
			results = append(results, result)
		}
		return results, nil
	}

	type job struct {
		Index int
		Blog  model.Blog
	}
	jobs := make(chan job)
	results := make([]ScanResult, len(blogs))

	g, gctx := errgroup.WithContext(ctx)

	for i := 0; i < workers; i++ {
		g.Go(func() error {
			workerDB, err := storage.OpenDatabase(gctx, db.Path())
			if err != nil {
				return err
			}
			defer func() {
				if err := workerDB.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "close: %v\n", err)
				}
			}()
			for item := range jobs {
				result, err := s.ScanBlog(gctx, workerDB, item.Blog)
				if err != nil {
					return fmt.Errorf("scan %s: %w", item.Blog.Name, err)
				}
				results[item.Index] = result
			}
			return nil
		})
	}

	g.Go(func() error {
		defer close(jobs)
		for index, blog := range blogs {
			select {
			case jobs <- job{Index: index, Blog: blog}:
			case <-gctx.Done():
				return gctx.Err()
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *Scanner) ScanBlogByName(ctx context.Context, db *storage.Database, name string) (*ScanResult, error) {
	blog, err := db.GetBlogByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if blog == nil {
		return nil, nil
	}
	result, err := s.ScanBlog(ctx, db, *blog)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func convertFeedArticles(blogID int64, articles []rss.FeedArticle) []model.Article {
	result := make([]model.Article, 0, len(articles))
	for _, article := range articles {
		result = append(result, model.Article{
			BlogID:        blogID,
			Title:         article.Title,
			URL:           article.URL,
			PublishedDate: article.PublishedDate,
			IsRead:        false,
			Categories:    article.Categories,
		})
	}
	return result
}

func convertScrapedArticles(blogID int64, articles []scraper.ScrapedArticle) []model.Article {
	result := make([]model.Article, 0, len(articles))
	for _, article := range articles {
		result = append(result, model.Article{
			BlogID:        blogID,
			Title:         article.Title,
			URL:           article.URL,
			PublishedDate: article.PublishedDate,
			IsRead:        false,
		})
	}
	return result
}
