-- +goose Up
CREATE TABLE content_taxonomy_terms (
    account_id BINARY(16) NOT NULL,
    content_id BINARY(16) NOT NULL,
    taxonomy_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT content_taxonomy_terms_primary_key
        PRIMARY KEY (account_id, content_id, taxonomy_id),
    CONSTRAINT content_taxonomy_terms_content_fk
        FOREIGN KEY (account_id, content_id)
        REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_taxonomy_terms_taxonomy_fk
        FOREIGN KEY (account_id, taxonomy_id)
        REFERENCES taxonomies (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX content_taxonomy_terms_taxonomy_index (account_id, taxonomy_id, content_id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE content_taxonomy_terms;
