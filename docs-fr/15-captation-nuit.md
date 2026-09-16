# Captation nuit du 15 au 16 septembre 2026 — amas 132-133 en continu

*Compte rendu de l'agent qui tient la station, à l'intention de `co-atc-local`.
Session demandée pour éprouver le détecteur de voix sur une nuit entière de parasite.*

## Ce qui a été fait

**23:01:02 → 03:00:00 CEST**, soit 3 h 59, sur les sept canaux de `amas-132-133`.

Sortie fichier **continue** (`split_on_transmission = false`, `continuous = true`)
ajoutée à côté du mélangeur, comme le 15/09 sur `orly-approche`. Pas de sortie par
transmission : c'est elle qui fragmente le direct.

Deux gabarits temporaires ont été créés pour la session, tous deux dérivés de
`amas-132-133.tmpl` par substitution contrôlée (garde sur le compte : 7 canaux,
7 mélangeurs, 7 sorties fichier pour l'un, 0 pour l'autre) :

- `amas-132-133-nuit-continu` — la captation ;
- `amas-132-133-nuit-reference` — les **mêmes sept canaux sans aucune sortie
  fichier**, pour encadrer la mesure sur le même groupe et la même nuit.

La station est revenue sur `decollages-atterrissages` à 03:06:23. Vérifié à 08h45 :
mode initial, **zéro sortie fichier** dans `rtl_airband.conf`, chaîne à l'antenne,
`accord: true`.

## Le matériel

**35 fichiers, ≈ 115 Mo**, servis sur
`http://macmini-fedora.lan/fichiers/transmissions/<freq>/`

| Fréquence | Fichiers | Volume |
|---|---|---|
| 132,275 | 5 | 17,1 Mo |
| 132,500 | 5 | 16,0 Mo |
| 132,733 | 5 | 16,2 Mo |
| 132,783 | 5 | 13,8 Mo |
| 132,825 | 5 | 16,9 Mo |
| 133,000 | 5 | 17,3 Mo |
| 133,250 | 5 | 17,6 Mo |

Nommage `c_AAAAMMJJ_HH.mp3`, découpage à l'heure : `..._23`, `..._00`, `..._01`,
`..._02`, plus un `..._03` de **une seconde** — le résidu entre 03:00:00 et la
bascule. **Ignorer les cinq fichiers `_03`.**

Le journal complet de la session :
`http://macmini-fedora.lan/fichiers/nuit/captation-nuit-20260915.log`

> ⚠️ **Durées.** Comme le 15/09, ce sont des MP3 à débit variable sans en-tête Xing :
> `ffprobe` annonce n'importe quoi. Il faut compter les trames — mais en **parcourant
> les longueurs de trame**, pas en cherchant les motifs de synchronisation à l'aveugle :
> un simple balayage des `0xFF` suivis de trois bits hauts double le compte sur ces
> fichiers (vérifié : 102 306 « trames » annoncées pour un fichier d'une heure, soit
> deux fois trop). Repère utile : le fichier `_23` commence à **23:01:02**, pas à 23:00.

## Les relevés salves.py

Tous sur `http://audio.lan/aero.mp3`, le flux réellement diffusé.

```
AVANT, groupe decollages-atterrissages  (22:45-22:47)
  #1  4 salves   mediane 2,9 s   <1 s : 25 %   ouvert 19,9 %
  #2  3 salves   mediane 1,9 s   <1 s : 33 %   ouvert 25,1 %
  #3  5 salves   mediane 2,6 s   <1 s : 20 %   ouvert 10,0 %

AVANT, amas-132-133 SANS enregistrement  (22:52-22:54)
  #1 12 salves   mediane 3,0 s   <1 s : 25 %   ouvert 47,2 %
  #2 10 salves   mediane 2,4 s   <1 s : 30 %   ouvert 46,2 %
  #3 13 salves   mediane 1,5 s   <1 s : 38 %   ouvert 47,8 %

PENDANT, amas-132-133 AVEC enregistrement continu  (23:03-23:05)
  #1  PLANTAGE de salves.py
  #2  PLANTAGE de salves.py
  #3 17 salves   mediane 0,3 s   <1 s : 59 %   ouvert 28,0 %

APRES, amas-132-133 SANS enregistrement  (03:04-03:06)
  #1  4 salves   mediane 0,8 s   <1 s : 50 %   ouvert 33,0 %
  #2  9 salves   mediane 1,2 s   <1 s : 44 %   ouvert 22,6 %
  #3  aucune salve

APRES, groupe decollages-atterrissages  (03:10-03:12)
  #1 #2 #3  aucune salve
```

## Ce que ça permet de conclure — et ce que ça ne permet pas

**Cette nuit ne tranche pas la question de la dégradation.** Il faut le dire net.

Le seul relevé valide *pendant* la captation donne une médiane de 0,3 s et 59 % de
salves sous la seconde : à première vue, la signature du défaut de fragmentation.
Mais les relevés *après*, **sans aucune sortie fichier**, sur le même groupe, donnent
0,8 s / 50 % puis 1,2 s / 44 % puis rien du tout. Et sur `decollages-atterrissages`
à 3 h du matin, les trois relevés ne trouvent **aucune salve** : les tours ne parlent
pas la nuit.

Trois raisons pour lesquelles la mesure ne conclut pas :

1. **Deux relevés sur trois ont planté** pendant la captation (voir ci-dessous), il
   ne reste qu'un point contre trois de chaque côté ;
2. **aucune paire n'est à la même heure** — 23 h contre 03 h, alors que le régime du
   parasite et le trafic réel changent complètement entre les deux ;
3. **l'indicateur lui-même perd son sens la nuit** : quand il n'y a pas de parole, ce
   que `salves.py` mesure, c'est la structure du bruit secteur, pas celle du flux.

**Ce qui reste établi** sur la sortie fichier continue, c'est la mesure du **15/09 en
journée** : 2 h 38, trois relevés par état, avant 2,6 s / 25 %, pendant 3,9 s / 15 %,
après 4,6 s / 22 % — aucune dégradation, la dispersion interne dépassant l'écart entre
les états. Cette conclusion-là tient.

**Pour une prochaine nuit**, intercaler un relevé de référence sans enregistrement
à 01 h et à 02 h, de façon à apparier chaque point *avec* à un point *sans* pris dans
la même heure. C'est la seule façon de mesurer quoi que ce soit dans ce régime.

## Le défaut de salves.py

```
AttributeError: 'NoneType' object has no attribute 'group'
  dur = float(re.search(r"Duration: (\d+):(\d+):([\d.]+)", txt).group(1)) * 3600 + ...
```

`ffprobe` n'a pas trouvé de champ `Duration` dans la capture — vraisemblablement un
fichier tronqué ou vide, la chaîne venant de redémarrer. Le script meurt au lieu de
signaler. **À durcir** : sortir proprement avec un message quand la durée est
introuvable, plutôt que de faire perdre un point de mesure.

## Deux découvertes utiles au chantier

**`filtre-voix` compte comme un auditeur Icecast, en permanence.** Il consomme
`aero.mp3` pour produire `aero-clair.mp3`. Le champ `auditeurs` de `/radio/etat` ne
tombe donc **jamais** à zéro tant que la chaîne tourne : le plancher est 1. Tout code
qui teste « personne n'écoute » doit raisonner sur **auditeurs − 1**. Ce piège a
failli annuler la session en silence.

**Le binaire Co-ATC compte lui aussi comme auditeur.** Pendant la préparation, il
tournait sur le M4 et tirait le flux via un `ffmpeg` (`192.168.1.28:64296 →
192.168.1.10:80`), en plus de sa connexion à tar1090 sur le port 8080. C'est visible
depuis la station avec `docker exec caddy netstat -tn`.

## Contexte réseau de la nuit

Dix chutes du lien Ethernet entre 22 h et 8 h 45, toutes suivies d'un retour en 4 à
5 secondes, et **le bail `192.168.1.10` a été repris à chaque fois** — la station
n'est jamais repassée en Wi-Fi. Le redémarrage complet du mesh Linksys de la veille a
réparé le pontage, pas le battement.

Sans effet sur les enregistrements : l'écriture est locale, elle ne traverse pas le
réseau. Mais c'est à savoir si un flux Icecast par canal est un jour tiré à distance
pendant une nuit entière.

## ADS-B

`/opt/adsb/globe_history` : 31 Mo, **aucune purge planifiée** — ni cron, ni minuteur
systemd, ni variable de rétention dans le compose. Les tranches ne risquent rien.
La chaîne ADS-B n'a pas été approchée.
