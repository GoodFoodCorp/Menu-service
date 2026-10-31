-- The 14 allergens of regulation (EU) 1169/2011 present in each dish
-- (codes, see domain.Allergens): shown to customers before they order.
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS allergens TEXT[] NOT NULL DEFAULT '{}';
