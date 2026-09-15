# Nuit du 15 au 16 septembre — ce que la captation a donné

*Travail mené en autonomie. 1 468 transmissions transcrites par trois modèles,
croisées avec l'historique ADS-B reconstitué. Données : `whisper-lab/nuit-*.json`.*

## 1. Le modèle est départagé — et ce n'est pas celui de `03-transcription.md`

Jusqu'ici, comparer deux modèles revenait à mesurer leur accord mutuel, ce qui ne prouve
rien quand ils partagent leur corpus d'affinage. **L'ADS-B tranche autrement** : le modèle
qui produit le plus d'indicatifs réellement présents dans le ciel gagne, et aucune
annotation n'est nécessaire.

| Fréquence | `large-v3-atco2` multilingue | `jacktol medium.en` |
|---|---|---|
| 124,350 Approche CDG + LB | **75 %** — 49,6 vrais | 71 % — 43,4 |
| 125,933 secteur en route | **78 %** — 14,0 | 64 % — 9,0 |
| 123,875 Orly Approche | **57 %** — 5,8 | 15 % — 0,8 |
| 124,625 Paris Contrôle DG | **57 %** — 9,8 | **−12 %** — −1,0 |
| 125,825 De Gaulle Approche | **49 %** — 14,8 | 38 % — 10,4 |
| **Total appariements vrais** | **94,0** | **62,6** |

**Le multilingue affiné ATC trouve 50 % d'indicatifs vérifiables de plus, sur les cinq
fréquences sans exception.** Et sur 124,625 le modèle anglais tombe **au niveau du
hasard** : ses appariements n'y valent pas mieux qu'un tirage au sort.

> **Conséquence pour Q1.** `03-transcription.md` désignait `jacktol/whisper-medium.en` comme
> candidat, et c'est ce que j'ai mesuré pendant une journée entière. **C'est le mauvais
> modèle.** Le bon est `sfabriece/whisper-large-v3-atco2-asr-mlx` — même famille d'affinage
> ATC, mais sur base `large-v3` multilingue. La branche anglaise de la stratégie A′ est
> tranchée.

## 2. La stratification verticale de la TMA, retrouvée par la radio

En passant la grammaire sur chaque fréquence séparément, les altitudes citées dessinent
l'étagement du contrôle parisien :

| Fréquence | Ce qu'on y entend | Ce que ça dit |
|---|---|---|
| **125,933** | FL380 ×7, FL370 ×4, FL400, FL430 | **espace supérieur, croisière** |
| **124,625** | FL190 ×9, FL260 ×5, FL250 — *aucune altitude basse* | descente haute |
| **124,350** | FL100 ×25, FL190 ×15, puis 3 000 à 5 000 ft | approche intermédiaire |
| **125,825** | FL70 ×11, 600 ft, **pistes 27R ×3, 26L, 27** | **approche finale** |
| **123,875** | 4 000 ft ×8, FL80, FL100 | Orly, terminal bas |

**L'ordre de descente se lit directement** : un avion passe du FL380 sur 125,933 au FL190
sur 124,625, au FL100 sur 124,350, au FL70 puis à la piste sur 125,825. Les mentions de
piste n'apparaissent que sur 125,825 — c'est bien elle qui pose les avions.

Et **125,933 est confirmée sur 131 transmissions** au lieu des 40 du premier essai : c'est
un secteur de contrôle en route en espace supérieur. Elle reste « non identifiée » au
catalogue.

## 3. Pourquoi 125,825 est la pire alors qu'elle est la plus fournie

49 % de précision, la dernière du lot, alors qu'elle porte 491 transmissions et **26 %
d'occupation** — bien au-delà des 3 à 5 % qu'`01-station.md` donne pour une fréquence de
contrôle ordinaire.

L'hypothèse la plus simple est que **c'est justement parce qu'elle est saturée** :
transmissions qui se chevauchent, débit rapide, collationnements serrés. **Non vérifié.**
Le test serait de mesurer la précision en fonction de l'occupation instantanée.

## 4. Deux trous dans la grammaire, trouvés par la mesure

**Les chiffres français n'existaient pas.** Sur 280 transmissions passées au modèle
français, la grammaire produisait **un seul** candidat d'indicatif — non par échec du
modèle, mais parce qu'elle ne connaissait que `one two three`. Ajout des chiffres et des
mots-clés français : `niveau`, `cap`, `piste`, `vitesse`, `fréquence`, `transpondeur`,
`pieds`, `nœuds`, `degrés`.

**Les nombres écrits en chiffres étaient ignorés.** Les modèles mélangent les deux formes
dans la même phrase — « Latour Five Eight Seven » à côté de « manoeuvre 470 Maya ». La
grammaire n'ouvrait un nombre que sur un chiffre épelé et jetait tous les autres. Après
correction, le modèle français passe de **1 à 151 candidats** sur 123,875, et y atteint
41 % de précision pour 2,9 appariements vrais — faible, mais plus nul.

## 5. Ce que la nuit n'a pas réglé

- **Le français reste sans réponse.** 2,9 appariements vrais sur 280 transmissions, et la
  seule fréquence mixte du lot était largement anglophone à cette heure-là. Rien ici ne
  renseigne sur l'aviation légère francophone, qui est le vrai sujet.
- **Aucune mesure de justesse absolue.** Un appariement d'indicatif dit que *ce
  numéro-là* était juste, pas que la phrase l'était. **Les 120 annotations restent le seul
  chemin vers un taux d'erreur de mots.**
- **La précision plafonne à 75-78 % sur les meilleures fréquences.** Une clairance fausse
  sur quatre reste inacceptable pour alimenter la carte.

## 6. Ce qui est désormais mesurable et ne l'était pas

La chaîne complète — découper, transcrire, analyser, apparier, comparer à l'ADS-B — tourne
sur commande et rend des chiffres avec leur témoin. **Changer un modèle, un seuil ou une
règle se juge maintenant en une minute**, sur 1 468 transmissions réelles, sans annoter
quoi que ce soit :

```bash
go run ./cmd/phraseology -capture <transcriptions.json> -adsb <adsb.json> \
                         -window 60 -min-digits 3 -seeds 8
```

C'est, rétrospectivement, l'apport le plus utile de la journée : pas un résultat, un
instrument.
