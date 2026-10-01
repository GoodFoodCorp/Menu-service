DROP TABLE IF EXISTS menu_plan_dishes;
DROP TABLE IF EXISTS menu_plans;
ALTER TABLE menu_items DROP COLUMN IF EXISTS ingredients;
ALTER TABLE menu_items ALTER COLUMN tenant_id SET NOT NULL;
