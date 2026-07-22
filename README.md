# Menu Service

Microservice Go gérant les **menus/articles par restaurant** pour Good Food 3.0.
Chaque article appartient à exactement un restaurant (tenant) : c'est la
frontière d'isolation multi-tenant.

## Architecture (Clean / Hexagonal)

```
internal/
├── domain/          # MenuItem (rattaché à un TenantID), erreurs typées, ports
├── application/     # Cas d'usage : menu public d'un resto, CRUD scopé au tenant
│                    #   du manager, seed d'un menu de départ par restaurant
├── adapter/
│   ├── http/        # chi, middleware JWT (HS256 partagé), DTO, OpenAPI /docs
│   ├── postgres/    # Repository pgx + migrations (index sur tenant_id)
│   └── authclient/  # Lit la liste des restaurants (tenants) pour le seed
└── config/          # Config typée depuis l'environnement
```

## Isolation multi-tenant

- **Client** : `GET /api/menu?restaurantId=<tenantId>` → articles *disponibles*
  du restaurant (public, sans authentification).
- **Franchisé** (`manager`) : ne voit et ne modifie que le menu de **son**
  restaurant, déterminé par le `tenant_id` de son JWT. Toute tentative de
  toucher au menu d'un autre restaurant renvoie `403`.

## Prérequis & lancement

```bash
docker network create microservices-net   # une fois (partagé)
cp .env.example .env                      # POSTGRES_PASSWORD + JWT_SECRET
                                          # ⚠️ JWT_SECRET identique au auth-service
docker compose up -d --build
```

Au démarrage, le service récupère les restaurants depuis auth-service et seede
un menu de départ (6 plats) pour chaque restaurant qui n'en a pas — chaque
article est une ligne distincte rattachée à son restaurant.

## Variables d'environnement

| Variable | Requis | Description |
|---|---|---|
| `POSTGRES_USER/PASSWORD/DB` | oui | Base dédiée `menu-db` (port hôte 5435) |
| `DATABASE_URL` | oui (hors docker) | Chaîne pgx |
| `JWT_SECRET` | oui | Secret HS256 **identique au auth-service** |
| `AUTH_SERVICE_URL` | non | Défaut `http://auth-service:8081` (pour le seed) |

## Endpoints (port 8085)

Doc interactive : `GET /docs`

| Méthode | Route | Rôle |
|---|---|---|
| GET | `/api/menu?restaurantId=` | public (client) |
| GET | `/api/menu/manage` | manager (son resto) |
| POST | `/api/menu` | manager (crée dans son resto) |
| PATCH | `/api/menu/{id}` | manager (son resto uniquement) |
| DELETE | `/api/menu/{id}` | manager (son resto uniquement) |
| GET | `/healthz`, `/readyz` | public (probes K8s) |

## Tests

```bash
go test ./internal/... -cover   # isolation multi-tenant couverte
```
