# Première mesure — mlx-whisper sur des transmissions réelles

*15 septembre 2026, Mac mini M4 Pro, 24 Gio. Banc hors dépôt :
`~/Dev/Aero/whisper-lab/`. Chiffres bruts dans `q2c-resultats.json`.*

> ⚠️ **Ce n'est pas une évaluation.** 16 fichiers, **non annotés**, pris dans l'ordre
> alphabétique et non au hasard — exactement le piège d'échantillonnage que
> `04-corpus.md` décrit. Aucun taux d'erreur de mots n'est calculé ici et aucun ne peut
> l'être. Ce document sert à répondre à Q2 (le modèle affiné se charge-t-il ?) et à
> dégrossir Q1. **La mesure qui décidera reste celle des 120 transmissions annotées.**

## Réponse à Q2 : oui, avec une manipulation

**`mlx-whisper` charge le modèle affiné de jacktol, après conversion.** Trois obstacles,
tous franchis, tous à documenter dans les instructions d'installation du fork :

1. **`convert.py` n'est pas dans le paquet pip `mlx-whisper`** (0.4.3). Le paquet ne
   contient que l'inférence. Il faut le dépôt `ml-explore/mlx-examples`, dossier
   `whisper/`. Dépendance non versionnée par pip.
2. **La conversion elle-même est triviale** : 10,5 s pour un `medium` en float16,
   1,42 Gio produits, code retour 0.
3. **Les deux dépôts d'Apple ne s'accordent pas sur le nom du fichier de poids.**
   `convert.py` écrit `model.safetensors` ; `mlx_whisper/load_models.py:29` cherche
   `weights.safetensors`, puis se rabat sur `weights.npz` et meurt en essayant d'ouvrir
   un safetensors comme une archive zip :
   `ValueError: [load_npz] Input must be a zip file`. Un `cp` suffit.

**Q2 est donc tranchée, et elle ne bloque plus Q1.** La stratégie A (aiguillage par
langue) reste sur la table.

Au passage, le `config.json` du modèle converti donne **`n_vocab: 51864`**. Le
vocabulaire multilingue de Whisper en compte 51 865. C'est le vocabulaire **anglais
seul** — confirmation par le fichier, et non par la documentation, que ce modèle ne peut
pas produire de français.

## Vitesse — l'écart est énorme

16 transmissions, 95,9 s d'audio au total.

| Modèle | n | Calcul médian | × temps réel (médian) | Calcul total |
|---|---|---|---|---|
| **jacktol ATC, medium.en, converti MLX** | 16 | **0,49 s** | **× 8,5** | **20,0 s** |
| `large-v3-turbo` (mlx-community) | 16 | 4,09 s | × 1,4 | 60,2 s |

Par fréquence :

| Modèle | Fréquence | × temps réel |
|---|---|---|
| jacktol | 132,500 (anglais) | **× 10,7** |
| jacktol | 129,525 (français) | × 7,1 |
| turbo | 132,500 | × 1,4 |
| turbo | 129,525 | × 1,6 |

**Six fois plus rapide.** C'est plus que l'écart de taille de modèle ne le laissait
attendre ; le décodage monolingue y contribue.

**Ce que ça dit du dimensionnement.** `01-station.md` donne 3 à 5 % d'occupation par
fréquence de contrôle. Sept canaux à 5 % font 0,35 × temps réel de parole à traiter.
Les deux modèles tiennent le débit. Ce n'est donc **pas** la vitesse qui décidera de
Q1 — c'est la qualité. Bonne nouvelle : elle libère le choix.

## Qualité — deux résultats opposés

### Sur l'anglais (132,500), jacktol est franchement meilleur

| | jacktol ATC | large-v3-turbo |
|---|---|---|
| | `descend flight level three five zero easy nine seven eight six` | `U-7 Fart level 350ה, Z9886` |
| | `go ahead sun express seven seven kilo` | `Go ahead, like this 7 to someone.` |
| | `hello romeo six five echo romeo flight level three eight zero direct to roger` | `Hello, runner 65, Mekaromia, bottom 380, direct posted.` |
| | `yeah easy five easy nine double eight six` | `Yeah, but in 5 is in 9886.` |

À gauche, de la phraséologie OACI bien formée. À droite, du charabia — une transmission
part même en caractères hébreux, une autre boucle sur « 7 and 7 and 7 and » vingt-deux
fois de suite, ce qui est la boucle de dégénérescence classique de Whisper.

Les deux modèles s'accordent sur `EZY9886` et sur `flight level 380` : cette part-là est
probablement juste. **Et elle confirme au passage que 132,500 porte du trafic commercial
en anglais**, ce que `06-catalogue.csv` donnait comme « non identifiée ».

### Sur le français (129,525), jacktol est dangereux

Rappel : ce sont des transmissions d'aéroclub à Chavenay, en français.

```
jacktol  « cleared to land runway zero five oscar kilo papa »
jacktol  « lufthansa one romeo romeo is ready for immediate clearance from the top
           of the base of the airbus in one three two two five three »
jacktol  « hotel xray victor alfa juliett direct to romeo alfa mike pushback to
           maintain level two hundred »
```

**Il n'y a ni Lufthansa, ni pushback, ni niveau 200 sur la fréquence d'un aéroclub des
Yvelines.** Le modèle invente. `03-transcription.md` prévoyait « au mieux une
translittération absurde » : c'est pire. La sortie est **bien formée, confiante, et
entièrement fausse** — un échec qui ressemble à une réussite. Un post-traitement qui la
reçoit n'a aucun moyen de la distinguer d'une bonne transcription.

**C'est l'argument le plus fort pour la stratégie A**, et aussi son plus grand risque :
l'aiguillage doit être fiable, sinon la panne est silencieuse.

## Un piège découvert, et qui aurait pu coûter cher

jacktol produit des noms de lieux **tchèques** sur les deux fréquences :

| Modèle | 129,525 (français) | 132,500 (anglais) |
|---|---|---|
| **jacktol** | `praha` ×1, `venox` ×2 | `praha` ×1, `ruzyne` ×1, `tepac` ×1 |
| large-v3-turbo | aucun | aucun |

`ruzyne` est l'aéroport de Prague (LKPR). Deux explications possibles :

- **une réception lointaine réelle** — un avion au FL380 a un horizon radio de plusieurs
  centaines de milles nautiques ;
- **un biais d'affinage** — jacktol est affiné sur ATCO2 et **UWB-ATCC, le corpus de
  l'université de Bohême-Occidentale**, c'est-à-dire du contrôle aérien tchèque.

**Le test qui tranche était à portée de main et il tranche :** « praha » apparaît sur un
fichier de **Chavenay**, un aéroclub des Yvelines où l'on parle français, et
`large-v3-turbo` ne produit ces mots **nulle part**. C'est donc le biais d'affinage. Le
modèle rabat ce qu'il ne comprend pas sur le vocabulaire de son corpus d'entraînement.

> **Conséquence pour l'exploitation.** Un modèle affiné sur de l'ATC tchèque hallucinera
> des points de report tchèques sur notre bande. Le post-traitement devra donc contraindre
> la sortie contre **les balises et points de report réellement présents dans les 100 NM**
> — que Co-ATC charge déjà (43 balises, mesuré le 15/09) — exactement comme il contraint
> déjà les indicatifs contre la liste ADS-B courante. Voir Q6 et `07-carte-openai.md`.
>
> Et une conséquence pour l'évaluation : **compter les hallucinations de toponymes est un
> indicateur en soi**, moins coûteux à mesurer qu'un WER et plus parlant pour ce produit.

## Ce que cette mesure ne dit pas

- Aucun WER, sur aucune des deux langues. Il faut la vérité terrain.
- Rien sur le taux de reconnaissance des indicatifs et des chiffres, qui est le critère
  qui compte (`04-corpus.md`, point 3).
- Rien sur `large-v3-turbo` correctement amorcé : ces essais ont été faits **sans prompt
  initial**. L'amont en utilise un (`prompts/transcription_prompt.txt`) et il demande
  explicitement la phraséologie et les chiffres en toutes lettres. Une part de l'écart
  observé pourrait s'expliquer ainsi. **À refaire avec amorce avant toute conclusion sur B.**
- Rien sur l'aiguillage lui-même, qui est le cœur de la stratégie A.

## Suite

1. Annoter 120 transmissions (`04-corpus.md`). Rien ne se décide avant.
2. Refaire le comparatif **avec l'amorce de l'amont**, pour ne pas condamner B à tort.
3. Mesurer l'a priori par fréquence comme aiguilleur : sur les dossiers de langue connue,
   quelle est la part de transmissions où la détection contredit l'a priori ?
