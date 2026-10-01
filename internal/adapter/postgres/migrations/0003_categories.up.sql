-- Dish categories are a network-wide taxonomy managed by head office
-- (siège), not per-restaurant — every franchisee picks from the same list
-- when authoring a dish. menu_items.category stays a free string (no FK):
-- deleting a category never breaks dishes already using it.
CREATE TABLE IF NOT EXISTS menu_categories (
    id         UUID PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    sort_order INT  NOT NULL DEFAULT 0
);

INSERT INTO menu_categories (id, name, sort_order) VALUES
    (gen_random_uuid(), 'Burgers', 1),
    (gen_random_uuid(), 'Pizzas', 2),
    (gen_random_uuid(), 'Salades', 3),
    (gen_random_uuid(), 'Desserts', 4),
    (gen_random_uuid(), 'Boissons', 5)
ON CONFLICT (name) DO NOTHING;
