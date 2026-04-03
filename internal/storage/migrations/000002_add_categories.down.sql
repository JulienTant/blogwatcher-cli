-- SQLite does not support DROP COLUMN prior to 3.35.0.
-- Recreate the table without the categories column.
CREATE TABLE articles_backup (
    id INTEGER PRIMARY KEY,
    blog_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    published_date TIMESTAMP,
    discovered_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (blog_id) REFERENCES blogs(id)
);

INSERT INTO articles_backup SELECT id, blog_id, title, url, published_date, discovered_date, is_read FROM articles;

DROP TABLE articles;

ALTER TABLE articles_backup RENAME TO articles;
