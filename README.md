# Menu Service

Microservice **Go** gérant les **menus et articles de chaque restaurant**. Chaque
article appartient à exactement un restaurant : c'est la frontière d'isolation
multi-tenant.

| | |
|---|---|
| **Langage / techno** | Go 1.26, chi (routeur), pgx (PostgreSQL), zerolog, golang-migrate |
| **Base de données** | PostgreSQL (port hôte `5435`) |
| **Port HTTP** | `8085` |
| **Documentation API** | http://localhost:8085/docs |

---

## Architecture — Clean / Hexagonale

```
cmd/main.go                # Démarrage, migrations, seed, injection des dépendances
internal/
├── domain/                # MenuItem (rattaché à un TenantID), erreurs typées, ports
├── application/           # Cas d'usage : menu public d'un restaurant,
│                          # CRUD scopé au restaurant du franchisé, seed initial
├── adapter/
│   ├── http/              # Routeur chi, middleware JWT, DTO, OpenAPI (/docs)
│   ├── postgres/          # Repository pgx + migrations (index sur tenant_id)
│   └── authclient/        # Lit la liste des restaurants pour le seed
└── config/                # Configuration typée depuis l'environnement
```

---

## Fonctionnalités

### Côté client (public)
- **Consulter le menu d'un restaurant** (`?restaurantId=`) — seuls les articles
  **disponibles** sont retournés, sans authentification (le storefront l'affiche
  avant connexion)

### Côté franchisé
- **Lister son menu complet**, y compris les articles rendus indisponibles
- **Créer** un article (nom, description, prix, catégorie, emoji, disponibilité)
- **Modifier** un article
- **Supprimer** un article
- Rendre un article **indisponible** : il disparaît du menu client mais reste
  visible en gestion

### Seed automatique
Au démarrage, le service récupère la liste des restaurants auprès de
**`franchise-service`** et crée un **menu de départ (6 plats)** pour chaque
restaurant qui n'en a pas encore. Chaque article est une ligne distincte
rattachée à son restaurant — les restaurants ne partagent aucune donnée.

### Isolation multi-tenant
Un franchisé ne peut **ni voir ni modifier** le menu d'un autre restaurant : le
restaurant est déduit du `tenant_id` de son JWT, jamais d'un paramètre client.
Toute tentative renvoie `403`.

---

## Endpoints

| Méthode | Route | Accès |
|---|---|---|
| GET | `/api/menu?restaurantId=` | public |
| GET | `/api/menu/manage` | `manager` (son restaurant) |
| POST | `/api/menu` | `manager` (crée dans son restaurant) |
| PATCH | `/api/menu/{id}` | `manager` (le sien uniquement) |
| DELETE | `/api/menu/{id}` | `manager` (le sien uniquement) |
| GET | `/healthz`, `/readyz` | public (sondes) |

---

## Lancement

```bash
docker network create microservices-net   # une seule fois, partagé
cp .env.example .env                      # renseigner POSTGRES_PASSWORD et JWT_SECRET
docker compose up -d --build
```

⚠️ `JWT_SECRET` doit être **identique** à celui de `auth-service`.
`franchise-service` doit tourner pour que le seed des menus fonctionne.

### Variables d'environnement

| Variable | Requis | Description |
|---|---|---|
| `PORT` | non (8085) | Port HTTP |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | oui | Base dédiée `menu-db` |
| `DATABASE_URL` | oui | Chaîne pgx (le compose la construit pour le conteneur) |
| `JWT_SECRET` | oui | Secret HS256 partagé avec `auth-service` |
| `FRANCHISE_SERVICE_URL` | non | Défaut `http://franchise-service:8089` (seed) |
| `LOG_LEVEL` | non (info) | `debug`, `info`, `warn`, `error` |

---

## Tests

```bash
go test ./internal/... -cover     # couverture ~80 % sur la couche application
go vet ./... && gofmt -l .
```

Couvrent notamment l'isolation multi-tenant (un franchisé ne peut pas modifier le
menu d'un autre restaurant) et l'idempotence du seed.

> ⚠️ **Aucune CI n'est configurée sur ce projet** — les tests doivent être lancés
> manuellement.
