package e2e

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "blogwatcher-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmp, "blogwatcher-cli")
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, "../cmd/blogwatcher-cli")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	if err := os.RemoveAll(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup temp dir: %v\n", err)
	}
	os.Exit(code)
}

// cliOpts builds args and env depending on the config mode ("flags" or "env").
type cliOpts struct {
	mode   string // "flags" or "env"
	dbPath string
}

// run executes the CLI binary. Positional args go in args. Named options go in
// opts as "key=value" pairs — they become --key value in flag mode, or
// BLOGWATCHER_KEY=value env vars in env mode.
func (c *cliOpts) run(t *testing.T, args []string, opts map[string]string) (stdout, stderr string, exitCode int) {
	t.Helper()

	var cmdArgs []string
	var extraEnv []string

	// DB path goes before the subcommand (persistent flag).
	if c.mode == "flags" {
		cmdArgs = append(cmdArgs, "--db", c.dbPath)
	} else {
		extraEnv = append(extraEnv, "BLOGWATCHER_DB="+c.dbPath)
	}

	// Positional args first (includes the subcommand name).
	cmdArgs = append(cmdArgs, args...)

	// Named options go after positional args so subcommand flags work.
	for k, v := range opts {
		if c.mode == "flags" {
			cmdArgs = append(cmdArgs, "--"+k)
			if v != "" {
				cmdArgs = append(cmdArgs, v)
			}
		} else {
			envKey := "BLOGWATCHER_" + strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
			if v == "" {
				v = "true" // boolean flags
			}
			extraEnv = append(extraEnv, envKey+"="+v)
		}
	}

	cmd := exec.CommandContext(context.Background(), binaryPath, cmdArgs...)
	cmd.Env = append(os.Environ(), append(extraEnv, "NO_COLOR=1")...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("exec error: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

func (c *cliOpts) ok(t *testing.T, args []string, opts map[string]string) string {
	t.Helper()
	stdout, stderr, code := c.run(t, args, opts)
	require.Equal(t, 0, code, "expected exit 0\nargs: %v\nstdout:\n%s\nstderr:\n%s", args, stdout, stderr)
	return stdout
}

func (c *cliOpts) fail(t *testing.T, args []string, opts map[string]string) (string, string) {
	t.Helper()
	stdout, stderr, code := c.run(t, args, opts)
	require.NotEqual(t, 0, code, "expected non-zero exit\nargs: %v\nstdout:\n%s", args, stdout)
	return stdout, stderr
}

// normalize replaces dynamic values with stable placeholders and sorts
// article blocks so output is deterministic regardless of scan order.
func normalize(s, baseURL string) string {
	s = strings.ReplaceAll(s, baseURL, "{{SERVER}}")
	s = regexp.MustCompile(`\[(\d+)\]`).ReplaceAllString(s, "[ID]")
	s = regexp.MustCompile(`Last scanned: \d{4}-\d{2}-\d{2} \d{2}:\d{2}`).ReplaceAllString(s, "Last scanned: {{TIMESTAMP}}")
	s = regexp.MustCompile(`Marked article \d+`).ReplaceAllString(s, "Marked article ID")
	s = regexp.MustCompile(`Article \d+ is`).ReplaceAllString(s, "Article ID is")
	s = regexp.MustCompile(`Article \d+ not`).ReplaceAllString(s, "Article ID not")
	s = sortArticleBlocks(s)
	return s
}

// sortArticleBlocks detects article listing output (blocks starting with
// "  [ID]") and sorts them alphabetically by title so the comparison is
// stable regardless of insertion order.
func sortArticleBlocks(s string) string {
	lines := strings.Split(s, "\n")

	// Find runs of article blocks. Each block starts with "  [ID]" and
	// continues with "       " indented detail lines + a blank separator.
	type block struct {
		lines []string
		key   string // the title line for sorting
	}

	var result []string
	var blocks []block
	var cur []string
	inArticles := false

	// normalizeBlock strips trailing blank lines and adds exactly one.
	normalizeBlock := func(lines []string) []string {
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return append(lines, "")
	}

	flushBlocks := func() {
		if len(blocks) == 0 {
			return
		}
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].key < blocks[j].key
		})
		for _, b := range blocks {
			result = append(result, normalizeBlock(b.lines)...)
		}
		blocks = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "  [ID]") {
			inArticles = true
			// Start a new article block.
			if len(cur) > 0 {
				blocks = append(blocks, block{lines: cur, key: cur[0]})
			}
			cur = []string{line}
		} else if inArticles && (strings.HasPrefix(line, "       ") || line == "") {
			cur = append(cur, line)
		} else {
			// End of article listing.
			if len(cur) > 0 {
				blocks = append(blocks, block{lines: cur, key: cur[0]})
				cur = nil
			}
			flushBlocks()
			inArticles = false
			result = append(result, line)
		}
	}
	if len(cur) > 0 {
		blocks = append(blocks, block{lines: cur, key: cur[0]})
	}
	flushBlocks()

	return strings.Join(result, "\n")
}

// checkOutput compares normalized output against expected/<name>.txt.
// With UPDATE_EXPECTED=1, writes actual output to disk instead.
func checkOutput(t *testing.T, name string, raw string, baseURL string) {
	t.Helper()
	actual := normalize(raw, baseURL)
	path := filepath.Join("expected", name+".txt")

	if os.Getenv("UPDATE_EXPECTED") == "1" {
		err := os.WriteFile(path, []byte(actual), 0644)
		require.NoError(t, err)
		return
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err, "missing expected file: %s (run with UPDATE_EXPECTED=1 to generate)", path)
	assert.Equal(t, string(data), actual, "output mismatch for %s\nRun with UPDATE_EXPECTED=1 to update", name)
}

// startTestServer serves real RSS/Atom/HTML fixtures from testdata/.
func startTestServer(t *testing.T) string {
	t.Helper()

	atomFeed, err := os.ReadFile("testdata/go_blog.atom")
	require.NoError(t, err)
	rssFeed, err := os.ReadFile("testdata/github_blog.rss")
	require.NoError(t, err)
	rustHTML, err := os.ReadFile("testdata/rust_blog.html")
	require.NoError(t, err)
	goHTMLTemplate, err := os.ReadFile("testdata/go_blog.html")
	require.NoError(t, err)

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	baseURL := "http://" + listener.Addr().String()

	goHTML := []byte(strings.ReplaceAll(string(goHTMLTemplate), "{{FEED_URL}}", baseURL+"/go/feed.atom"))

	mux := http.NewServeMux()

	writeOrFail := func(w http.ResponseWriter, data []byte) {
		_, err := w.Write(data)
		assert.NoError(t, err, "write response")
	}

	mux.HandleFunc("/go/feed.atom", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		writeOrFail(w, atomFeed)
	})
	mux.HandleFunc("/go/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/go/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		writeOrFail(w, goHTML)
	})
	mux.HandleFunc("/github/feed/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		writeOrFail(w, rssFeed)
	})
	mux.HandleFunc("/github/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/github/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		writeOrFail(w, []byte(`<!DOCTYPE html><html><head>
<link type="application/rss+xml" rel="alternate" href="/github/feed/" title="GitHub Blog" />
</head><body><h1>GitHub Blog</h1></body></html>`))
	})
	mux.HandleFunc("/rust/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rust/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		writeOrFail(w, rustHTML)
	})

	go func() {
		err := http.Serve(listener, mux)
		if !errors.Is(err, net.ErrClosed) {
			assert.NoError(t, err, "http.Serve")
		}
	}()
	t.Cleanup(func() {
		err := listener.Close()
		if !errors.Is(err, net.ErrClosed) {
			assert.NoError(t, err, "listener.Close")
		}
	})

	return baseURL
}

func TestE2E(t *testing.T) {
	baseURL := startTestServer(t)

	for _, mode := range []string{"flags", "env"} {
		t.Run(mode, func(t *testing.T) {
			c := &cliOpts{
				mode:   mode,
				dbPath: filepath.Join(t.TempDir(), "test.db"),
			}

			// ── Add blogs ──
			out := c.ok(t, []string{"add", "go-blog", baseURL + "/go/"}, map[string]string{
				"feed-url": baseURL + "/go/feed.atom",
			})
			checkOutput(t, "01_add_go_blog", out, baseURL)

			out = c.ok(t, []string{"add", "github-blog", baseURL + "/github/"}, nil)
			checkOutput(t, "02_add_github_blog", out, baseURL)

			out = c.ok(t, []string{"add", "rust-blog", baseURL + "/rust/"}, map[string]string{
				"scrape-selector": ".post-list td a[href]",
			})
			checkOutput(t, "03_add_rust_blog", out, baseURL)

			// ── Duplicate add fails ──
			_, stderr := c.fail(t, []string{"add", "go-blog", "http://other.example.com"}, nil)
			checkOutput(t, "04_add_duplicate_name", stderr, baseURL)

			_, stderr = c.fail(t, []string{"add", "other-name", baseURL + "/go/"}, nil)
			checkOutput(t, "05_add_duplicate_url", stderr, baseURL)

			// ── List blogs ──
			out = c.ok(t, []string{"blogs"}, nil)
			checkOutput(t, "06_blogs", out, baseURL)

			// ── Scan all ──
			out = c.ok(t, []string{"scan"}, nil)
			checkOutput(t, "07_scan_all", out, baseURL)

			// ── Scan single (no new) ──
			out = c.ok(t, []string{"scan", "go-blog"}, nil)
			checkOutput(t, "08_scan_single_no_new", out, baseURL)

			// ── Scan nonexistent blog ──
			_, stderr = c.fail(t, []string{"scan", "nope"}, nil)
			checkOutput(t, "09_scan_nonexistent", stderr, baseURL)

			// ── Scan silent ──
			out = c.ok(t, []string{"scan"}, map[string]string{"silent": ""})
			checkOutput(t, "10_scan_silent", out, baseURL)

			// ── Scan with workers ──
			c.ok(t, []string{"scan"}, map[string]string{"workers": "1"})

			// ── Articles (unread) ──
			out = c.ok(t, []string{"articles"}, nil)
			checkOutput(t, "11_articles_unread", out, baseURL)

			// ── Articles filtered by blog ──
			out = c.ok(t, []string{"articles"}, map[string]string{"blog": "go-blog"})
			checkOutput(t, "12_articles_filter_blog", out, baseURL)

			// ── Read / unread cycle ──
			articlesOut := c.ok(t, []string{"articles"}, nil)
			id := extractFirstID(t, articlesOut)

			out = c.ok(t, []string{"read", id}, nil)
			checkOutput(t, "13_read_article", out, baseURL)

			out = c.ok(t, []string{"read", id}, nil)
			checkOutput(t, "14_read_already_read", out, baseURL)

			out = c.ok(t, []string{"unread", id}, nil)
			checkOutput(t, "15_unread_article", out, baseURL)

			out = c.ok(t, []string{"unread", id}, nil)
			checkOutput(t, "16_unread_already_unread", out, baseURL)

			// ── Read invalid/nonexistent ID ──
			c.fail(t, []string{"read", "abc"}, nil)

			_, stderr = c.fail(t, []string{"read", "99999"}, nil)
			checkOutput(t, "17_read_nonexistent", stderr, baseURL)

			// ── Read-all ──
			out = c.ok(t, []string{"read-all"}, map[string]string{"yes": ""})
			checkOutput(t, "18_read_all", out, baseURL)

			out = c.ok(t, []string{"articles"}, nil)
			checkOutput(t, "19_articles_after_read_all", out, baseURL)

			// ── Articles --all shows read ──
			out = c.ok(t, []string{"articles"}, map[string]string{"all": ""})
			checkOutput(t, "20_articles_all", out, baseURL)

			// ── Remove blog ──
			out = c.ok(t, []string{"remove", "rust-blog"}, map[string]string{"yes": ""})
			checkOutput(t, "21_remove_blog", out, baseURL)

			out = c.ok(t, []string{"blogs"}, nil)
			checkOutput(t, "22_blogs_after_remove", out, baseURL)

			// ── Remove nonexistent ──
			_, stderr = c.fail(t, []string{"remove", "nope"}, map[string]string{"yes": ""})
			checkOutput(t, "23_remove_nonexistent", stderr, baseURL)
		})
	}
}

func TestImportOPML(t *testing.T) {
	baseURL := startTestServer(t)

	for _, mode := range []string{"flags", "env"} {
		t.Run(mode, func(t *testing.T) {
			c := &cliOpts{
				mode:   mode,
				dbPath: filepath.Join(t.TempDir(), "test.db"),
			}

			// Write an OPML file with feeds pointing at the test server.
			opmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
    <head><title>Test Subscriptions</title></head>
    <body>
        <outline text="Tech" title="Tech">
            <outline type="rss" text="Go Blog" title="Go Blog" xmlUrl="%s/go/feed.atom" htmlUrl="%s/go/"/>
            <outline type="rss" text="GitHub Blog" title="GitHub Blog" xmlUrl="%s/github/feed/" htmlUrl="%s/github/"/>
        </outline>
    </body>
</opml>`, baseURL, baseURL, baseURL, baseURL)
			opmlPath := filepath.Join(t.TempDir(), "subs.opml")
			err := os.WriteFile(opmlPath, []byte(opmlContent), 0o644)
			require.NoError(t, err)

			// Import the OPML file.
			out := c.ok(t, []string{"import", opmlPath}, nil)
			checkOutput(t, "30_import_opml", out, baseURL)

			// Verify blogs appear in list.
			out = c.ok(t, []string{"blogs"}, nil)
			checkOutput(t, "31_import_blogs_listed", out, baseURL)

			// Re-import the same file -- all should be skipped as duplicates.
			out = c.ok(t, []string{"import", opmlPath}, nil)
			checkOutput(t, "32_import_opml_duplicates", out, baseURL)

			// Import a nonexistent file should fail.
			c.fail(t, []string{"import", "/nonexistent/file.opml"}, nil)
		})
	}
}

func extractFirstID(t *testing.T, output string) string {
	t.Helper()
	re := regexp.MustCompile(`\[(\d+)\]`)
	matches := re.FindStringSubmatch(output)
	require.NotEmpty(t, matches, "no article ID found in output:\n%s", output)
	return matches[1]
}
