-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='product_image' AND column_name='position') THEN
        ALTER TABLE product_image ADD COLUMN position INTEGER;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_product_image_position ON product_image (product_id, position);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_product_image_position;
ALTER TABLE product_image DROP COLUMN IF EXISTS position;
-- +goose StatementEnd
