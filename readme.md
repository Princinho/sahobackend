# API Documentation — Boutique Vitrine SAHO

> Backend Go · Gin · MongoDB  
> Toutes les routes sont préfixées par la base URL du serveur (ex: `http://localhost:8080`).

---

## Table des matières

- [Configuration & Authentification](#configuration--authentification)
- [Auth](#auth)
- [Produits](#produits)
- [Catégories](#catégories)
- [Demandes de devis](#demandes-de-devis)
- [Demandes de produit sur mesure](#demandes-de-produit-sur-mesure)
- [Routes admin (protégées)](#routes-admin-protégées)
  - [Produits (admin)](#produits-admin)
  - [Catégories (admin)](#catégories-admin)
  - [Demandes de devis (admin)](#demandes-de-devis-admin)
  - [Demandes de produit sur mesure (admin)](#demandes-de-produit-sur-mesure-admin)
  - [Utilisateurs (admin)](#utilisateurs-admin)
- [Codes d'erreur](#codes-derreur)

---

## Configuration & Authentification

### Tokens

L'API utilise une stratégie **Access Token + Refresh Token** :

| Token | Format | Durée | Transport |
|---|---|---|---|
| Access Token | JWT signé | Courte durée | Header `Authorization: Bearer <token>` |
| Refresh Token | Opaque | 30 jours | Cookie `HttpOnly; Secure; SameSite=None; Path=/auth/refresh` |

### Header d'authentification

```http
Authorization: Bearer <access_token>
```

### Exemple d'intercepteur Axios (React)

```js
axios.defaults.baseURL = 'https://votre-domaine.com';
axios.defaults.withCredentials = true; // indispensable pour le cookie refresh

axios.interceptors.request.use(config => {
  const token = /* votre store */ getAccessToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// Refresh automatique sur 401
axios.interceptors.response.use(null, async error => {
  if (error.response?.status === 401) {
    const { data } = await axios.post('/auth/refresh');
    setAccessToken(data.accessToken);
    return axios(error.config);
  }
  return Promise.reject(error);
});
```

> **Bonne pratique :** stocker l'access token en mémoire (Zustand, Context) plutôt que dans `localStorage` pour limiter l'exposition aux attaques XSS.

---

## Auth

### `POST /auth/login`

Connecte un utilisateur et retourne un access token. Le refresh token est posé automatiquement en cookie.

**Body (JSON)**

```json
{
  "email": "admin@example.com",
  "password": "motdepasse"
}
```

| Champ | Type | Requis | Description |
|---|---|---|---|
| `email` | string | ✅ | Email de l'utilisateur |
| `password` | string | ✅ | Mot de passe |

**Réponse `200`**

```json
{ "access_token": "eyJ..." }
```

**Erreurs**

| Code | Cas |
|---|---|
| `400` | Body invalide |
| `401` | Email ou mot de passe incorrect |
| `403` | Compte désactivé |

---

### `POST /auth/refresh`

Renouvelle l'access token à partir du cookie `refresh_token`. Le cookie doit être envoyé automatiquement (`withCredentials: true`).

**Réponse `200`**

```json
{ "accessToken": "eyJ..." }
```

**Erreurs**

| Code | Cas |
|---|---|
| `401` | Cookie absent, token invalide ou expiré |
| `403` | Compte désactivé |

---

### `POST /auth/logout`

Révoque le refresh token côté serveur et efface le cookie.

**Réponse `200`**

```json
{ "ok": true }
```

---

## Produits

### `GET /products`

Liste paginée des produits. Par défaut, seuls les produits non désactivés (`isDisabled: false`) sont retournés.

**Query params**

| Param | Type | Défaut | Description |
|---|---|---|---|
| `page` | number | `1` | Numéro de page |
| `limit` | number | `20` | Résultats par page (max : 100) |
| `category` | string | — | Filtrer par **slug** de catégorie |
| `isTrending` | boolean | — | `true` pour les produits mis en avant |
| `isDisabled` | boolean | `false` | Inclure les produits désactivés |
| `sort` | string | `name_asc` | `price_asc` \| `price_desc` \| `stock_asc` \| `stock_desc` |

**Réponse `200`**

```json
{
  "items": [ /* Product[] */ ],
  "page": 1,
  "limit": 20,
  "total": 150,
  "category": "meubles",
  "sort": "price_asc",
  "ts": "2025-01-01T12:00:00Z"
}
```

**Objet `Product`**

```json
{
  "id": "665f...",
  "name": "Table basse",
  "slug": "table-basse",
  "price": 45000,
  "quantity": 10,
  "categoryIds": ["665f..."],
  "imageUrls": ["https://storage.googleapis.com/..."],
  "materials": ["bois", "métal"],
  "colors": ["noir", "blanc"],
  "description": "Description courte",
  "descriptionFull": "Description longue complète",
  "dimensions": "120x60x40 cm",
  "weight": "12 kg",
  "isTrending": false,
  "isDisabled": false
}
```

---

## Catégories

### `GET /categories`

Liste paginée des catégories.

**Query params**

| Param | Type | Défaut | Description |
|---|---|---|---|
| `page` | number | `1` | Numéro de page |
| `limit` | number | `50` | Résultats par page (max : 200) |
| `q` | string | — | Recherche insensible à la casse sur le nom |
| `isActive` | boolean | — | Filtrer par statut actif/inactif |

**Réponse `200`**

```json
{
  "items": [ /* Category[] */ ],
  "page": 1,
  "limit": 50,
  "total": 12
}
```

---

### `GET /categories/:id`

Retourne une catégorie par son ObjectID MongoDB.

**Réponse `200`**

```json
{
  "id": "665f...",
  "name": "Meubles",
  "slug": "meubles",
  "description": "Toute notre gamme de meubles",
  "isActive": true,
  "imageUrl": "https://storage.googleapis.com/..."
}
```

**Erreurs** : `400` ID invalide · `404` Introuvable

---

### `GET /categories/slug/:slug`

Identique à `GET /categories/:id` mais par slug.

```
GET /categories/slug/meubles
```

---

## Demandes de devis

### `POST /quote-requests`

Permet à un visiteur non connecté de soumettre une demande de devis avec un ou plusieurs produits.

**Body (JSON)**

```json
{
  "fullName": "Jean Dupont",
  "email": "jean@example.com",
  "phone": "+228 90 00 00 00",
  "country": "Togo",
  "city": "Lomé",
  "address": "Rue des Fleurs, 12",
  "message": "Je souhaite un devis pour livraison rapide.",
  "items": [
    { "productId": "665f...", "quantity": 2 },
    { "productId": "665f...", "quantity": 1 }
  ]
}
```

| Champ | Type | Requis | Description |
|---|---|---|---|
| `fullName` | string | ✅ | Nom complet |
| `email` | string | ✅ | Email valide |
| `phone` | string | ❌ | Téléphone |
| `country` | string | ❌ | Pays |
| `city` | string | ❌ | Ville |
| `address` | string | ❌ | Adresse |
| `message` | string | ❌ | Message libre |
| `items` | array | ✅ | Au moins 1 article |
| `items[].productId` | string | ✅ | ObjectID du produit |
| `items[].quantity` | number | ✅ | Quantité (>= 1) |

**Réponse `201`**

```json
{
  "id": "665f...",
  "message": "Your quote request has been submitted. We will get back to you shortly."
}
```

**Erreurs** : `400` Données invalides ou `productId` inexistant · `500` Erreur serveur

---

## Demandes de produit sur mesure

### `POST /product-requests`

Permet à un visiteur non connecté de soumettre une demande de fabrication d'un produit sur mesure. Requête **multipart/form-data**.

> ⚠️ Ne pas forcer le `Content-Type` — laisser Axios/fetch le déduire automatiquement depuis le `FormData`.

**Champs multipart**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `data` | string (JSON) | ✅ | Données de la demande sérialisées en JSON |
| `image` | File | ❌ | Image ou PDF de référence (jpg, png, webp, pdf) |

**Champ `data` (JSON)**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `fullName` | string | ✅ | Nom complet |
| `email` | string | ✅ | Email valide |
| `phone` | string | ❌ | Téléphone |
| `company` | string | ❌ | Entreprise |
| `country` | string | ❌ | Pays |
| `city` | string | ❌ | Ville |
| `description` | string | ✅ | Description du produit souhaité (min. 5, max. 8000 caractères) |
| `quantity` | number | ❌ | Quantité souhaitée (défaut : `1`) |
| `desiredDeadline` | string (ISO date) | ❌ | Date limite souhaitée |
| `budget` | string | ❌ | Budget indicatif |
| `referenceUrl` | string | ❌ | URL de référence |

**Exemple React**

```js
const formData = new FormData();
formData.append('data', JSON.stringify({
  fullName: 'Jean Dupont',
  email: 'jean@example.com',
  description: 'Table basse en bois avec plateau en verre fumé, 120x60 cm.',
  quantity: 2,
  budget: '150 000 FCFA',
}));
if (referenceFile) formData.append('image', referenceFile);

await axios.post('/product-requests', formData);
```

**Réponse `201`**

```json
{
  "id": "665f...",
  "message": "Your product request has been submitted. We will get back to you shortly."
}
```

**Erreurs** : `400` Données invalides ou fichier refusé · `500` Erreur serveur

---

## Routes admin (protégées)

> Toutes les routes ci-dessous requièrent le header `Authorization: Bearer <access_token>`.  
> Elles sont préfixées par `/admin`.

---

### Produits (admin)

#### `POST /admin/products/add`

Crée un nouveau produit. Requête **multipart/form-data**.

> ⚠️ Ne pas forcer le `Content-Type` — laisser Axios/fetch le déduire automatiquement depuis le `FormData`.

**Champs multipart**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `data` | string (JSON) | ✅ | Données du produit sérialisées en JSON |
| `images` | File[] | ❌ | Une ou plusieurs images (max `MAX_PROD_IMAGES`, généralement 4) |

**Champ `data` (JSON)**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `name` | string | ✅ | Nom du produit (min. 3 caractères) |
| `price` | number | ✅ | Prix (> 0) |
| `quantity` | number | ✅ | Stock (>= 0) |
| `categoryIds` | string[] | ✅ | Au moins 1 ObjectID de catégorie |
| `materials` | string[] | ❌ | Matériaux |
| `colors` | string[] | ❌ | Couleurs disponibles |
| `description` | string | ❌ | Description courte |
| `descriptionFull` | string | ❌ | Description longue |
| `dimensions` | string | ❌ | Ex : `"120x60x40 cm"` |
| `weight` | string | ❌ | Ex : `"12 kg"` |
| `isTrending` | boolean | ❌ | Mis en avant (défaut : `false`) |
| `isDisabled` | boolean | ❌ | Désactivé (défaut : `false`) |

> Le `slug` est **auto-généré** à partir du `name` côté serveur. Ne pas l'envoyer.

**Exemple React**

```js
const formData = new FormData();
formData.append('data', JSON.stringify({
  name: 'Table basse',
  price: 45000,
  quantity: 10,
  categoryIds: ['665f...'],
  description: 'Belle table en bois massif',
}));
files.forEach(f => formData.append('images', f));

await axios.post('/admin/products/add', formData);
```

**Réponse `201`** : L'objet `Product` complet.

**Erreurs** : `400` Données invalides · `409` Slug déjà existant · `500` Erreur serveur

---

#### `PATCH /admin/products/update/:id`

Met à jour un produit existant. Requête **multipart/form-data**.  
Permet d'ajouter de nouvelles images et de supprimer des images existantes en une seule requête.

**Champs multipart**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `data` | string (JSON) | ✅ | Champs à modifier (tous optionnels) |
| `images` | File[] | ❌ | Nouvelles images à ajouter |

**Champ `data` — champs spécifiques à la mise à jour**

| Champ | Type | Description |
|---|---|---|
| `removedImagesUrls` | string[] | URLs des images à supprimer (doivent appartenir au produit) |
| `categoryIds` | string[] | Remplacement complet des catégories |
| `name`, `price`, `quantity`, `slug`, `description`, `descriptionFull`, `materials`, `colors`, `dimensions`, `weight`, `isTrending`, `isDisabled` | — | Mêmes champs que la création, tous optionnels |

> ⚠️ Le nombre total d'images (`existantes - supprimées + nouvelles`) ne doit pas dépasser `MAX_PROD_IMAGES`.

**Exemple React**

```js
const formData = new FormData();
formData.append('data', JSON.stringify({
  price: 49000,
  removedImagesUrls: ['https://storage.googleapis.com/.../ancienne.jpg'],
}));
newFiles.forEach(f => formData.append('images', f));

await axios.patch(`/admin/products/update/${id}`, formData);
```

**Réponse `200`**

```json
{ "ok": true }
```

---

### Catégories (admin)

#### `POST /admin/categories`

Crée une nouvelle catégorie. Requête **multipart/form-data**.

**Champs multipart**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `data` | string (JSON) | ✅ | `{ name, slug?, description?, isActive? }` |
| `image` | File | ❌ | Image de la catégorie |

> Le `slug` est auto-généré depuis le `name` s'il n'est pas fourni.

**Exemple React**

```js
const formData = new FormData();
formData.append('data', JSON.stringify({ name: 'Meubles', isActive: true }));
if (imageFile) formData.append('image', imageFile);

await axios.post('/admin/categories', formData);
```

**Réponse `201`**

```json
{ "id": "665f..." }
```

---

#### `PATCH /admin/categories/:id`

Met à jour une catégorie. Requête **multipart/form-data**. Tous les champs sont optionnels.

**Champ `data` (JSON)**

| Champ | Type | Description |
|---|---|---|
| `name` | string | Nouveau nom |
| `slug` | string | Nouveau slug |
| `description` | string | Nouvelle description |
| `isActive` | boolean | Nouveau statut |

> Envoyer un fichier dans le champ `image` remplace l'ancienne image (supprimée de GCS automatiquement).

**Réponse `200`**

```json
{ "ok": true }
```

---

#### `DELETE /admin/categories/:id`

Supprime la catégorie et son image associée sur Google Cloud Storage.

**Réponse `200`**

```json
{ "ok": true }
```

---

### Demandes de devis (admin)

#### `GET /admin/quote-requests`

Liste paginée des demandes, triées du plus récent au plus ancien.

**Query params**

| Param | Type | Défaut | Description |
|---|---|---|---|
| `page` | number | `1` | Numéro de page |
| `limit` | number | `20` | Résultats par page (max : 100) |
| `status` | string | — | Filtrer : `NEW` \| `IN_PROGRESS` \| `QUOTED` \| `REJECTED` \| `CLOSED` |

**Réponse `200`**

```json
{
  "items": [ /* QuoteRequest[] */ ],
  "page": 1,
  "limit": 20,
  "total": 34
}
```

---

#### `GET /admin/quote-requests/:id`

Retourne le détail complet d'une demande, incluant les notes admin.

**Objet `QuoteRequest`**

```json
{
  "id": "665f...",
  "fullName": "Jean Dupont",
  "email": "jean@example.com",
  "phone": "+228 90 00 00 00",
  "country": "Togo",
  "city": "Lomé",
  "address": "Rue des Fleurs, 12",
  "message": "...",
  "items": [
    {
      "productId": "665f...",
      "quantity": 2,
      "productName": "Table basse",
      "productSlug": "table-basse",
      "unitPrice": 45000
    }
  ],
  "status": "NEW",
  "quotedAt": null,
  "notes": [
    {
      "id": "665f...",
      "authorId": "665f...",
      "authorEmail": "admin@example.com",
      "content": "Voici le devis en pièce jointe.",
      "createdAt": "2025-01-01T12:00:00Z",
      "quotePdf": {
        "publicUrl": "https://storage.googleapis.com/...",
        "objectName": "quotes/665f.../devis.pdf",
        "mimeType": "application/pdf",
        "sizeBytes": 204800
      }
    }
  ],
  "createdAt": "2025-01-01T10:00:00Z",
  "updatedAt": "2025-01-01T12:00:00Z"
}
```

**Statuts possibles**

| Statut | Description |
|---|---|
| `NEW` | Nouvelle demande, non traitée |
| `IN_PROGRESS` | En cours de traitement |
| `QUOTED` | Devis envoyé au client |
| `REJECTED` | Demande refusée |
| `CLOSED` | Dossier clôturé |

---

#### `PATCH /admin/quote-requests/:id/status`

Modifie le statut d'une demande. Le passage à `QUOTED` horodate automatiquement le champ `quotedAt`.

**Body (JSON)**

```json
{ "status": "IN_PROGRESS" }
```

**Réponse `200`**

```json
{ "ok": true }
```

**Erreurs** : `400` Statut non reconnu · `404` Demande introuvable

---

#### `POST /admin/quote-requests/:id/notes`

Ajoute une note admin à une demande. Si la demande est au statut `NEW`, elle passe automatiquement à `IN_PROGRESS`.  
Requête **multipart/form-data**.

**Champs multipart**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `data` | string (JSON) | ✅ | `{ "content": "Texte de la note" }` |
| `pdf` | File | ❌ | Devis PDF en pièce jointe |

**Exemple React**

```js
const formData = new FormData();
formData.append('data', JSON.stringify({ content: 'Voici le devis.' }));
if (pdfFile) formData.append('pdf', pdfFile);

await axios.post(`/admin/quote-requests/${id}/notes`, formData);
```

**Réponse `201`** : L'objet `Note` créé (voir structure dans `GET /admin/quote-requests/:id`).

---

### Demandes de produit sur mesure (admin)

#### `GET /admin/product-requests`

Liste paginée des demandes de produit sur mesure, triées du plus récent au plus ancien.

**Query params**

| Param | Type | Défaut | Description |
|---|---|---|---|
| `page` | number | `1` | Numéro de page |
| `limit` | number | `20` | Résultats par page |
| `status` | string | — | Filtrer : `NEW` \| `IN_PROGRESS` \| `ANSWERED` \| `REJECTED` \| `CLOSED` |
| `email` | string | — | Filtrer par email exact |
| `q` | string | — | Recherche sur `fullName`, `email`, `company`, `description` |

**Réponse `200`**

```json
{
  "items": [ /* ProductRequest[] */ ],
  "page": 1,
  "limit": 20,
  "total": 18
}
```

---

#### `GET /admin/product-requests/:id`

Retourne le détail complet d'une demande de produit sur mesure.

**Objet `ProductRequest`**

```json
{
  "id": "665f...",
  "fullName": "Jean Dupont",
  "email": "jean@example.com",
  "phone": "+228 90 00 00 00",
  "company": "ACME",
  "country": "Togo",
  "city": "Lomé",
  "description": "Table basse en bois avec plateau en verre fumé.",
  "quantity": 2,
  "desiredDeadline": "2025-06-01T00:00:00Z",
  "budget": "150 000 FCFA",
  "referenceUrl": "https://example.com/ref",
  "referenceImage": {
    "imageUrl": "https://storage.googleapis.com/...",
    "objectName": "product-requests/665f.../ref.jpg",
    "mimeType": "image/jpeg",
    "sizeBytes": 102400,
    "fileName": "ref.jpg",
    "uploadedAt": "2025-01-01T10:00:00Z"
  },
  "status": "NEW",
  "notes": [
    {
      "id": "665f...",
      "authorId": "665f...",
      "authorEmail": "admin@example.com",
      "content": "Nous pouvons réaliser cette pièce.",
      "createdAt": "2025-01-01T12:00:00Z",
      "attachment": {
        "imageUrl": "https://storage.googleapis.com/...",
        "objectName": "product-requests/665f.../note-attachment.pdf",
        "mimeType": "application/pdf",
        "sizeBytes": 204800,
        "fileName": "devis.pdf",
        "uploadedAt": "2025-01-01T12:00:00Z"
      }
    }
  ],
  "answeredAt": null,
  "createdAt": "2025-01-01T10:00:00Z",
  "updatedAt": "2025-01-01T12:00:00Z"
}
```

**Statuts possibles**

| Statut | Description |
|---|---|
| `NEW` | Nouvelle demande, non traitée |
| `IN_PROGRESS` | En cours de traitement |
| `ANSWERED` | Réponse envoyée au client |
| `REJECTED` | Demande refusée |
| `CLOSED` | Dossier clôturé |

---

#### `PATCH /admin/product-requests/:id/status`

Modifie le statut d'une demande. Le passage à `ANSWERED` horodate automatiquement le champ `answeredAt`.

**Body (JSON)**

```json
{ "status": "IN_PROGRESS" }
```

**Réponse `200`**

```json
{ "ok": true }
```

**Erreurs** : `400` Statut non reconnu · `404` Demande introuvable

---

#### `POST /admin/product-requests/:id/notes`

Ajoute une note admin à une demande. Si la demande est au statut `NEW`, elle passe automatiquement à `IN_PROGRESS`.  
Requête **multipart/form-data**.

**Champs multipart**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `data` | string (JSON) | ✅ | `{ "content": "Texte de la note" }` |
| `file` | File | ❌ | Pièce jointe (image ou PDF) |

**Exemple React**

```js
const formData = new FormData();
formData.append('data', JSON.stringify({ content: 'Nous pouvons réaliser cette pièce.' }));
if (attachmentFile) formData.append('file', attachmentFile);

await axios.post(`/admin/product-requests/${id}/notes`, formData);
```

**Réponse `201`** : L'objet `Note` créé (voir structure dans `GET /admin/product-requests/:id`).

---

### Utilisateurs (admin)

#### `POST /admin/users`

Crée un nouveau compte administrateur. Réservé aux utilisateurs ayant le rôle `ADMIN`.

**Body (JSON)**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `email` | string | ✅ | Email valide |
| `password` | string | ✅ | Mot de passe (min. 8 caractères) |

**Réponse `201`**

```json
{
  "id": "665f...",
  "email": "nouveau@example.com",
  "role": "ADMIN",
  "isActive": true,
  "createdAt": "2025-01-01T10:00:00Z",
  "updatedAt": "2025-01-01T10:00:00Z"
}
```

**Erreurs** : `400` Données invalides · `403` Non autorisé (rôle insuffisant) · `500` Erreur serveur

---

#### `POST /admin/users/me/password`

Permet à l'administrateur connecté de changer son propre mot de passe. Révoque tous les refresh tokens existants et efface le cookie — une reconnexion est requise.

**Body (JSON)**

| Champ | Type | Requis | Description |
|---|---|---|---|
| `currentPassword` | string | ✅ | Mot de passe actuel |
| `newPassword` | string | ✅ | Nouveau mot de passe |

**Réponse `200`**

```json
{ "ok": true }
```

**Erreurs** : `400` Données invalides · `401` Mot de passe actuel incorrect · `500` Erreur serveur

---

## Codes d'erreur

Format de toutes les réponses d'erreur :

```json
{
  "error": "message descriptif",
  "field": "slug"
}
```

> `field` est optionnel et précise le champ concerné (ex: lors d'un conflit de slug).

| Code | Signification | Action recommandée |
|---|---|---|
| `400` | Données invalides / champ manquant | Afficher `error` à l'utilisateur |
| `401` | Non authentifié ou token expiré | Appeler `/auth/refresh`, puis retenter |
| `403` | Compte désactivé | Afficher un message, déconnecter l'utilisateur |
| `404` | Ressource introuvable | Afficher une page 404 |
| `409` | Conflit (slug déjà existant) | Proposer un autre nom ou slug |
| `500` | Erreur serveur interne | Afficher un message générique, logger |

---

*Généré à partir du code source — `main.go`, `controllers/`, `models/`, `dto/`*  
*Dernière mise à jour : ajout de `product-requests` (public + admin) et gestion des utilisateurs admin.*