-- Per-restaurant dish recipes: stock item (Stock-service id) + quantity per
-- dish. Keyed by tenant so a global dish can have a different recipe in each
-- restaurant. Stock item name/unit are denormalised for display only.
CREATE TABLE IF NOT EXISTS menu_item_recipes (
    tenant_id       UUID NOT NULL,
    menu_item_id    UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    stock_item_id   UUID NOT NULL,
    stock_item_name TEXT NOT NULL,
    unit            TEXT NOT NULL,
    quantity        NUMERIC(12,3) NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (tenant_id, menu_item_id, stock_item_id)
);
