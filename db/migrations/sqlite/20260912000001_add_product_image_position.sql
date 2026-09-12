-- +goose Up
-- +goose StatementBegin
ALTER TABLE product_image ADD COLUMN position INTEGER;
CREATE INDEX idx_product_image_position ON product_image (product_id, position);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_product_image_position;
ALTER TABLE product_image DROP COLUMN position;
-- +goose StatementEnd
