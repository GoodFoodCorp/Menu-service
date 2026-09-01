-- Optional dish/menu photo, stored inline as a data: URL. No object storage
-- in this stack yet — the frontend downsizes/compresses the picture before
-- upload, and the application layer caps the string length (~3MB decoded).
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS image_data_url TEXT;
ALTER TABLE menu_plans ADD COLUMN IF NOT EXISTS image_data_url TEXT;
