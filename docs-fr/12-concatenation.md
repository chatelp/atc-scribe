# La concaténation aide-t-elle ? — non. Et ce qu'on a trouvé à la place

*15 septembre 2026. 384 transcriptions. Chiffres bruts : `whisper-lab/q20-resultats.json`.*

## La question

`03-transcription.md` propose : *« sur une transmission courte, allonger la fenêtre
d'analyse en concaténant la transmission précédente de la même fréquence peut aider »*.
Jamais testé. Et la piste est plus fondée qu'il n'y paraît : **en production, Co-ATC
reçoit un flux Icecast continu, pas des fichiers découpés.** Nos essais sur clips isolés
sont donc moins fidèles à la réalité que la concaténation.

## Le protocole

Une heure contiguë du 12/09, 14h00-15h00 : **133 transmissions sur 127,750 (Orly Départs)
et 175 sur 129,525 (Chavenay)**, dans l'ordre réel. 24 transmissions cibles par fréquence,
quatre variantes, deux modèles.

| Variante | Ce qu'on donne au modèle |
|---|---|
| `solo` | la transmission seule — la référence |
| `ctx1` | la précédente collée devant |
| `ctx2` | les deux précédentes collées devant |
| `gap1` | la précédente **plus le silence réel qui les sépare**, plafonné à 2 s |

L'horodatage du nom étant écrit à la fermeture du squelch,
`début = horodatage − durée`, donc `écart = début_cible − horodatage_précédente`.
`gap1` est la variante la plus proche du flux de production.

**Extraction de la cible** : on ne garde que les segments horodatés de Whisper dont la fin
tombe après le début de la transmission cible. Sans ça on mesurerait « plus d'audio donne
plus de texte ».

## Le résultat : nul, voire négatif

Accord entre les deux modèles ATC, en **F1 sur les mots** (0 = rien en commun) :

| Fréquence | `solo` | `ctx1` | `gap1` | `ctx2` |
|---|---|---|---|---|
| 127,750 Orly Départs | **0,16** | 0,19 | 0,18 | 0,17 |
| 129,525 Chavenay | **0,08** | 0,07 | 0,10 | 0,04 |

Tout est dans le bruit. Aucune variante ne dépasse `solo` de façon significative.

Ce qui bouge, en revanche :

- **Le temps de calcul explose** sur le modèle anglais à Chavenay : 1,73 s en `solo`,
  **13,67 s en `ctx1`**, 17,21 s en `ctx2`. Le repli de température, encore.
- **Le contexte déborde sur la cible.** Exemples relevés — `solo` : « descending level
  three zero from base one six mike three » ; `gap1` : « descending one zero zero **tomti
  air france one zero eight five** ». Le point de report et l'indicatif viennent de la
  transmission précédente. On ne gagne pas du contexte, on gagne de la contamination.

**Conclusion : la piste est fermée.** Elle reste peut-être valable pour la *détection de
langue* — allonger la fenêtre pour décider `fr`/`en` est un problème différent de
transcrire — mais pas pour la transcription elle-même.

## Ce qu'on a trouvé à la place, et qui compte davantage

En calculant l'accord entre **deux modèles ATC indépendants sur le même audio**, une
différence apparaît, et elle est nette :

| Fréquence | Type de trafic | n | **F1 médian** | Au-dessus de 0,30 |
|---|---|---|---|---|
| **132,500** | secteur en route, avions au FL380 | 24 | **0,37** | **15 / 24** |
| 127,750 | Orly Départs, terminal bilingue | 24 | 0,16 | 4 / 24 |
| 129,525 | Chavenay, aéroclub français | 25 | 0,16 | 4 / 25 |

**Plus du double sur l'en route.** Deux modèles entraînés séparément, sur des corpus
différents, avec des bases différentes — l'un multilingue, l'autre anglais seul —
convergent sur le trafic en route et divergent partout ailleurs.

Deux modèles qui s'accordent peuvent se tromper ensemble, mais **deux modèles qui
divergent ne peuvent pas avoir raison tous les deux**. Un F1 de 0,16 signifie qu'un mot
sur six est commun : au moins l'un des deux est très largement faux, et probablement les
deux.

### Ce que ça veut dire

**Ce n'est pas « aucun modèle ne marche ».** C'est : *les modèles marchent sur le trafic
en route et échouent sur le trafic terminal et l'aviation légère.*

C'est une nouvelle mitigée :

- **Bonne** pour le produit. Co-ATC croise la carte et la radio ; le trafic qui peuple la
  carte autour de la station est massivement du trafic de ligne en montée, croisière et
  approche. C'est là que les modèles tiennent.
- **Mauvaise** pour l'apport revendiqué. `00-mission.md` met le bilinguisme au centre, et
  le français de cette bande est très majoritairement de l'aviation légère et des tours de
  terrain — précisément le régime où tout s'effondre. La stratégie C, l'affinage maison sur
  de l'ATC francophone, cesse d'être « un horizon » pour devenir **la seule réponse
  connue** à ce problème.

### Ce qui reste à vérifier avant d'y croire

- **L'hypothèse n'est pas démontrée, seulement indiquée.** L'accord entre modèles est un
  indice, pas une mesure de justesse. Le jeu de 120 annoté tranchera.
- **La cause n'est pas établie.** Signal plus propre en vue directe depuis le FL380 ?
  Phraséologie OACI plus stricte ? Débit plus lent ? Locuteurs plus entraînés ? Les
  quatre à la fois ? On ne le sait pas, et ça se teste.
- **Aucune fréquence d'approche de grand terrain n'a été mesurée** — elles ne sont pas
  enregistrées (Q17). Or l'approche est le cas intermédiaire qui départagerait les
  hypothèses : phraséologie stricte comme l'en route, mais avions bas et proches comme le
  terminal. **C'est l'argument le plus fort en faveur de la captation demandée en
  `11-demande-station.md`.**
