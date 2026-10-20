# Orapa

Le jeu de déduction **Orapa Mine** en ligne, à deux, avec pendule. Chacun cache cinq gemmes dans sa mine ; à tour de rôle on
envoie des ondes dans la mine de l'autre (elles rebondissent sur les gemmes et en prennent la couleur), on sonde des cases, et
le premier qui propose la bonne disposition gagne.

En ligne : [orapa.eriphe.com](https://orapa.eriphe.com). On joue en invité, ou avec son Compte Eriphe pour garder ses parties.

## Règles retenues

- Mine de 10 × 8, 36 points d'entrée (1–18 en haut et à droite, A–R en bas et à gauche).
- Cinq gemmes, les pièces d'un tangram : grand triangle blanc, grand triangle bleu (base 4), triangle jaune (2 × 2),
  losange blanc (carré posé sur la pointe), parallélogramme rouge (3 de long, retournable).
- Pose : coins sur les points de la grille, jamais deux gemmes côte à côte par un côté, chacune visible en ligne droite
  depuis un bord. La pose se fait en même temps pour les deux, en 3 minutes.
- Onde : tout droit ; une face en diagonale la fait tourner de 90°, une face droite la renvoie d'où elle vient. Sa couleur
  est le mélange des couleurs touchées.
- À son tour : une onde, un sondage de case (couleur seulement), ou une proposition. Une proposition fausse fait perdre ;
  si le premier joueur trouve, l'autre a une dernière chance d'égaliser.
- Pendule par joueur avec incrément (5+3, 10+5, 15+10, 25+15). Le temps écoulé fait perdre.

## Architecture

Un seul binaire Go sert l'API, les WebSockets et le front (Vite + React) :

```
cmd/server/        point d'entrée (variables d'environnement, arrêt propre)
internal/game/     moteur de règles pur : mine, gemmes, trajet des ondes, poses, partie à deux
internal/server/   HTTP, WebSocket par partie, SQLite, comptes OIDC
web/               front : mine en SVG, pose des gemmes, journal, pendules
deploy/nas/        stack Compose Manager du NAS
```

- **Moteur** sans I/O ni concurrence, l'heure passée en paramètre ; testé à ~98 % (dont la réversibilité de tous les
  trajets sur des poses tirées au hasard).
- **Parties** en mémoire, une par verrou, enregistrées entières dans SQLite à chaque coup. Au redémarrage, les parties
  reprennent et le temps d'arrêt n'est décompté à personne.
- **Vues par joueur** : chacun voit sa pose et le trajet des ondes envoyées dans sa mine, jamais ceux de l'adversaire ;
  tout est dévoilé à la fin.
- **Comptes** : méthode au choix de l'hébergeur (gamekit/auth) ; en OIDC, jeton d'accès vérifié par la JWKS, `iss`/`aud` épinglés. La place
  d'un joueur est tenue par un jeton de place (seul son SHA-256 est gardé), ou retrouvée par son compte.

## Développement

```sh
go test ./...                                   # moteur + serveur
go run ./cmd/server                             # :8080, données dans ./data
cd web && npm install && npm run dev            # :5173, /api et /ws vers :8080
```

Variables : `ADDR` (`:8080`), `DATA_DIR` (`data`), `STATIC_DIR` (front compilé), `ALLOWED_ORIGINS` (origines
WebSocket en plus du même hôte).

### Comptes (facultatifs)

On joue toujours en invité ; un compte sert à garder ses parties. La méthode se choisit à l'hébergement, par
[gamekit/auth](https://github.com/alexis-marcel/gamekit) :

| Variable | Rôle | Défaut |
|---|---|---|
| `AUTH_MODE` | `none` (pas de comptes), `oidc` (Authentik, Keycloak, Authelia, Google…) `proxy` (reverse proxy qui authentifie déjà) ou `local` (comptes intégrés, pages `/compte/`) | `oidc` si `OIDC_ISSUER` est donné |
| `GUEST_PLAY` | `false` : un compte est exigé pour créer ou rejoindre une partie | `true` |
| `ACCOUNT_NAME` | nom du fournisseur dans l'interface (« Se connecter avec … ») | « Se connecter » |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_AUDIENCE` | fournisseur OIDC ; client public avec PKCE, redirections `/auth/callback` et `/auth/silent` | client : `orapa`, audience : le client |
| `TRUSTED_PROXIES`, `PROXY_USER_HEADER`, `PROXY_NAME_HEADER`, `PROXY_LOGIN_URL`, `PROXY_LOGOUT_URL` | mode `proxy` : voir [gamekit](https://github.com/alexis-marcel/gamekit#auth--comptes-au-choix-de-celui-qui-héberge) | `Remote-User`, `Remote-Name` |
| `SIGNUP` | mode `local` : `open`, `invite` (lien d'un administrateur) ou `closed` ; le premier compte créé est administrateur | `invite` |

## Déploiement

À chaque push sur `main`, l'action « Image Docker » publie `ghcr.io/alexis-marcel/orapa` (amd64 et arm64). `deploy/nas/deploy.sh` (ou « Update stack » dans Unraid) récupère la nouvelle image et relance la stack.
