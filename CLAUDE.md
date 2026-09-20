# atc-scribe — instructions permanentes

> **Disposition du dépôt.** Ce dépôt **est** un fork de `yegors/co-atc` : son historique
> est celui de l'amont, qui reste accessible via le remote `upstream`. Le travail se fait
> sur la branche `local`.
>
> Publié depuis le 20/09 : **https://github.com/chatelp/atc-scribe**, public. La branche
> `local` pousse vers `origin/main` — `main` est ce que voit un visiteur, `local` est le
> nom de travail. Attention en lançant `gh` : sans `-R chatelp/atc-scribe` il résout
> parfois sur le remote `upstream` et échoue en 404.
>
> - `docs/` appartient à l'amont — **on n'y touche pas**. `docs/LOCAL-STT.md` en particulier
>   est le plan de transcription locale écrit par l'auteur amont, et il nous sert de référence.
> - `docs-fr/` est **notre dossier de chantier**, en français. Voir `docs-fr/README.md`.
> - Ce fichier a remplacé le `CLAUDE.md` de l'amont, conservé à l'identique dans
>   `docs-fr/CLAUDE-amont.md`.

Tu travailles sur un **fork de [Co-ATC](https://github.com/yegors/co-atc)** adapté à une
station de réception aéronautique privée à Fontenay-le-Fleury (Yvelines, France).

**Lis `docs-fr/00-mission.md` en premier.** Les autres documents de `docs-fr/` sont la
référence : ils décrivent une installation qui existe et qui tourne, pas un projet
à imaginer.

## Ce que tu dois savoir avant de toucher à quoi que ce soit

1. **La station de réception est en production.** Elle alimente FlightAware et
   Flightradar24 en continu depuis des semaines, et son propriétaire l'écoute tous
   les jours. Tu peux lire tout ce que tu veux dessus ; **tu ne modifies sa
   configuration qu'après avoir prévenu et obtenu un accord explicite**, et jamais
   la chaîne ADS-B.
2. **Le développement se fait sur le Mac**, pas sur la station. La station est une
   source de données, pas un environnement de build.
3. **Tout ce qui est écrit dans `docs-fr/` a été vérifié sur la machine** le
   15 septembre 2026. Si tu constates un écart, c'est le document qui est périmé :
   corrige-le dans le même geste.
4. **Co-ATC amont, tu ne l'as pas lu.** Les notes de `docs-fr/02-co-atc-amont.md`
   viennent d'une lecture de la page du dépôt, pas du code. **Vérifie chaque
   affirmation dans les sources avant de t'appuyer dessus**, et corrige le document.

## Langue

Le projet est destiné à un dépôt public : **code, commentaires, noms de variables,
messages de commit et README en anglais**. Les documents de `docs-fr/` et les échanges
avec le propriétaire sont **en français**.

## Méthode

- **Mesurer avant de conclure.** C'est la culture de ce projet : chaque affirmation
  technique du dossier est adossée à un chiffre relevé. Tiens ce niveau. Si tu ne
  peux pas mesurer, dis que tu n'as pas mesuré.
- **Une décision structurante se discute avant d'être codée.** Elles sont listées
  dans `docs-fr/05-decisions.md` ; tiens ce fichier à jour, c'est la mémoire du chantier.
- **Pas de secret dans le dépôt.** Ni clé d'API, ni mot de passe Icecast, ni adresse
  exacte. La position de la station est déjà publique via FlightAware, mais la
  configuration d'exemple doit rester générique.
- **Le dépôt reste un fork, mais la rebasabilité n'a plus valeur de contrainte.**
  Mesuré le 16/09 (D19) : l'amont est silencieux depuis le 3 mai 2026, et notre empreinte
  sur ses fichiers est faible là où ça compterait — 1,7 % de `www/app.js`, 0,2 % de
  `internal/adsb/service.go`. **Modifie un fichier amont quand c'est la façon la plus
  simple d'écrire la chose** ; n'ajoute pas d'accesseur pour contourner une signature.
  En revanche, **garde identifiable ce qui est contribuable** : le sidecar de
  transcription, le correctif de rotation de base (Q26), l'authentification et l'état
  serveur doivent pouvoir partir en pull request tels quels.

## Commandes utiles

```bash
ssh pierre@192.168.1.10                       # la station, sans mot de passe
curl http://192.168.1.10:8080/data/aircraft.json    # ADS-B temps reel
curl -H 'Host: macmini-fedora.lan' http://192.168.1.10/radio/etat   # etat de la chaine VHF
```

Aucun sudo n'est requis sur la station pour `/opt/adsb`, les conteneurs et les
gabarits. Les modifications système (sysctl, systemd) demandent un mot de passe que
tu n'as pas — passe par le propriétaire.

## Le dossier `docs-fr/`

L'index et le rôle de chaque fichier sont dans `docs-fr/README.md`. Deux repères :

- **Commence par `docs-fr/00-mission.md`.**
- **`docs-fr/05-decisions.md` se tient à jour au fil de l'eau** — décisions prises d'un
  côté, questions ouvertes de l'autre. C'est la mémoire du chantier et la seule consigne
  non négociable de ce dossier.
