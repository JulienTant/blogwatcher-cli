### While developing

#### Tests, tests, tests!

Whenever possible, apply TDD. Write the test first, watch it fail, write the code, watch the test pass! This includes unit tests AND e2e tests!

#### Errors matter
You must never ignore an error. If this is a production error, log it. If it is a test error, assert/require on it.

```go
// BAD
_ = someFunction()

// GOOD
err := someFunction()
if err != nil {
    // ...
}
```

#### Use testify in tests
Tests will use the testify library, with `assert` and `require`. Do not use `t.` functions to do assertion.

```go
// BAD
if str == "" {
    t.Fatal("str is empty")
}

// GOOD
require.NotEmpty(t, str)
```

#### No raw SQL

Use `squirrel` for all queries. Never write raw SQL strings in Go code.

```go
// BAD
db.conn.QueryContext(ctx, "SELECT id, name FROM blogs WHERE id = ?", id)

// GOOD
sq.Select("id", "name").From("blogs").Where(sq.Eq{"id": id}).RunWith(db.conn).QueryRowContext(ctx)
```

Schema changes go in migration files under `internal/storage/migrations/` using `golang-migrate`. Migrations are `.sql` files embedded via `//go:embed` at compile time.

#### High standards

When we have to pick between the right way or the easy way, we always pick the right way. Never use `//nolint` directives to silence linters — fix the actual issue instead.

```go
client := &http.Client{Timeout: timeout}
// BAD
response, err := client.Get(blogURL)

// GOOD
req, err := http.NewRequestWithContext(ctx, http.MethodGet, blogURL, nil)
// handle error here
response, err := client.Do(req)
```

### After developing

- Ensure your files are correctly formatted by running `golangci-lint`. 
- Ensure the tests are ok by running `gotestsum`