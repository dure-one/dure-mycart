-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS new_carts (
    id         TEXT PRIMARY KEY NOT NULL,
    session_id TEXT NOT NULL,
    status     TEXT NOT NULL,
    total      TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT (datetime('now')),
    updated_at TIMESTAMP
);
CREATE INDEX idx_new_carts_session_id ON new_carts (session_id);

CREATE TABLE IF NOT EXISTS cart_items (
    id         TEXT PRIMARY KEY NOT NULL,
    cart_id    TEXT NOT NULL,
    product_id TEXT NOT NULL,
    quantity   INTEGER NOT NULL,
    price      TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT (datetime('now')),
    FOREIGN KEY (cart_id) REFERENCES new_carts(id) ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES product(id) ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX idx_cart_items_cart_id ON cart_items (cart_id);
CREATE INDEX idx_cart_items_product_id ON cart_items (product_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS new_carts;
-- +goose StatementEnd
