-- +goose Up
CREATE TABLE content_taxonomy_terms (
    account_id UUID NOT NULL,
    content_id UUID NOT NULL,
    taxonomy_id UUID NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT content_taxonomy_terms_primary_key
        PRIMARY KEY (account_id, content_id, taxonomy_id),
    CONSTRAINT content_taxonomy_terms_content_fk
        FOREIGN KEY (account_id, content_id)
        REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_taxonomy_terms_taxonomy_fk
        FOREIGN KEY (account_id, taxonomy_id)
        REFERENCES taxonomies (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX content_taxonomy_terms_taxonomy_index
    ON content_taxonomy_terms (account_id, taxonomy_id, content_id);

-- +goose Down
DROP TABLE content_taxonomy_terms;
