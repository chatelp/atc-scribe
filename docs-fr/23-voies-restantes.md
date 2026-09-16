# 23 — Où en est la chaîne, et ce qu'il reste comme voies

*16 septembre 2026, 13 h. Bilan de deux jours de mesures, puis les voies restantes pour
la reconnaissance vocale anglaise et l'enrichissement de co-atc — hors entraînement
d'un modèle français, exclu de la question.*

Ce document est en deux parties. **La première est technique et exhaustive** : l'état
mesuré de chaque étage, tout ce qui a été testé et éliminé avec les chiffres, ce qui
reste ouvert, et les voies proposées par un panel indépendant qui a lu le dossier et
le code puis tenté de réfuter chaque proposition. **La seconde, à la fin, est la
version courte** pour décider.

---

# Partie A — l'analyse détaillée

## A.1 La chaîne telle qu'elle tourne à 13 h

```
aero.mp3 (flux mixé, 4 à 7 canaux)
  → segmenteur énergie     silence_threshold 0,005 · 600 ms de silence termine · 400 ms min · préroll 200 ms
  → sidecar mlx-whisper    VAD Silero 0,5 / 250 ms / 300 ms · modèle EN = large-v3-atco2 · FR = bofenghuang (branché, non utilisé : language="en")
  → SQLite                 transcriptions(content, speaker_type, callsign) · clearances · phraseology_values
  → grammaire              parse.go : rôles, lettres, locuteur, clearances, normalisation
  → appariement ADS-B      3 chiffres min · fenêtre 60 s · ambigus refusés · pas de règle « à un près » · contexte 120 s (égalités seulement)
  → co-atc                 filtre « Heard on the radio » · panneau Radio par avion · valeurs en pastilles · texte des clearances
```

Tout tourne en local, sans clé, sans qu'un octet d'audio ni de texte quitte le Mac.
Le groupe écouté est choisi sur la station (`/opt/adsb/aero-mode`), aujourd'hui
`amas-132-133-nuit-reference` : les sept canaux de l'amas, sans sortie fichier.

## A.2 Ce que chaque étage vaut, mesuré

### Transcription

| grandeur | valeur | source |
|---|---|---|
| modèle anglais retenu | `sfabriece/whisper-large-v3-atco2-asr-mlx` | D12 — 94,0 appariements vrais contre 62,6 pour `jacktol medium.en`, sur les cinq fréquences sans exception |
| temps de calcul par transmission | 0,72 s médian (captation), 0,80–0,90 s observé en direct | doc 18, journal du 16/09 |
| première transcription après démarrage | 8,4 s (chargement du modèle) | journal du 16/09 |
| ouvertures de squelch sans parole, la nuit | **88,1 %** en moyenne, de 7,7 % à 99,1 % selon le canal | doc 21, 4 126 ouvertures sur 28 h |
| VAD contre modèle, VAD désactivé | 30/30 vides sur la classe « sans parole », 29/30 avec texte sur la classe « avec » | doc 21 |
| parasite secteur, peigne à 100 Hz | **exactement quatre canaux** : 132,733 · 132,825 · 133,000 · 133,250 | doc 21, spectre |
| silence numérique dans le flux mixé | 87,3 % des fenêtres de 20 ms à exactement zéro | doc 19 |
| dégénérescence en direct (groupe tours) | 2,2 % à 12 % selon l'échantillon, sur audio français passé au modèle anglais | doc 19 |

### Grammaire

| grandeur | valeur | source |
|---|---|---|
| transmissions livrant au moins une valeur structurée | **66 %** | doc 19 |
| transmissions portant une clearance | 4 % | doc 19 |
| clearances détectées → stockées, première heure | 6 → **1** ; les 5 perdues faute d'avion identifiable | doc 19 |
| locuteur attribué | 54 % (captation), 46–47 % (direct) — avant : 10 % | doc 19, D15 |
| justesse du locuteur | **non établie** ; contrôle d'alternance à 61 % / 59 % contre 50 %, sur 35 paires | Q20 |
| valeurs extraites sur la matinée (rattrapage) | FL 27 · fréquence 26 · altitude 15 · vitesse 15 · piste 12 · cap 9 · QNH 6 · transpondeur 1 | doc 19 |
| transmissions **sans aucun indicatif à trois chiffres** | **668 / 1 415 = 47 %** | doc 22 |

### Appariement

| grandeur | valeur | source |
|---|---|---|
| captation du 15/09, `atc-multi`, réglages de production | 144 appariés · 40,8 de hasard · **72 % au-dessus** · **103,2 vrais** | doc 19 (après retrait de la règle « à un près ») |
| groupe tours, 51 min | 20 appariés · 0,9 hasard · **96 %** · 19,1 vrais · 76 avions en fenêtre | doc 19 |
| groupe gros-porteurs, 51 min | 42 · 8,8 · **80 %** · 33,2 vrais · 93 avions | doc 19 |
| amas 132-133, 51 min *(10:48, fenêtre interrompue par 4 redémarrages)* | 44 · 3,5 · **93 %** · 40,5 vrais · 95 avions | doc 19 |
| amas 132-133, 51 min *(12:34, fenêtre propre : un seul démarrage, zéro erreur)* | 36 · 5,1 · **90 %** · 30,9 vrais · 100 avions · **214 transmissions, 4,2/min** | doc 19 |

> **Ce que la fenêtre propre change.** Le débit de l'amas était bien sous-estimé
> (175 → 214 transmissions), mais les appariements vrais tombent de 40,5 à 30,9 : ce
> n'était pas seulement l'interruption, c'était aussi l'heure. **Une fenêtre de 51 min
> ne suffit pas à classer finement deux groupes** — l'écart entre deux fenêtres du même
> groupe (40,5 / 30,9) est du même ordre que l'écart entre groupes. Ce qui tient :
> les tours rapportent moitié moins que les deux autres ; gros-porteurs et amas se
> valent en volume ; l'amas est nettement plus précis (90–93 % contre 80 %).
| indicatifs faits de lettres seules | **structurellement inappariables** : +0,30 < plancher 0,60 | doc 19 |
| opérateurs absents du lexique (`airlines.dat`, ~2014) | 23 / 77 = **30 %** | Q23 |
| effet du complément d'opérateurs relevé sur la station | **nul** sur les appariements (20/42/44 identiques) ; hasard 3,6 → 5,4 sur l'amas | Q23, Q25 |

### Écran

| ce que co-atc montre de la voix | depuis |
|---|---|
| texte brut et normalisé, locuteur en couleur, indicatif — liste par fréquence | amont |
| clearances par avion (heure, type, piste, statut, âge) | amont ; **texte** ajouté le 16/09 |
| filtre « Heard on the radio », 375 avions → 20 | 16/09, D16 |
| panneau Radio : transmissions rattachées à l'avion, valeurs en pastilles | 16/09 |
| champ `voice` dans l'API (transmissions, dernière heure, dernier texte) | 16/09 |

## A.3 Tout ce qui a été testé et éliminé

À ne pas re-proposer sans mesure nouvelle. Chaque ligne a coûté une mesure ; c'est
leur valeur.

| piste | mesure | résultat | référence |
|---|---|---|---|
| `jacktol medium.en` comme modèle anglais | ADS-B, 5 fréquences, témoin mélangé | 62,6 vrais contre 94,0 pour atco2 ; au hasard (−12 %) sur 124,625 | D12, doc 18 |
| amorce de transcription (`initial_prompt`) | 25 clips × 2 fréquences × 3 configurations | dégénérescence **×7**, fuite de l'amorce dans le texte | doc 10 |
| concaténer les transmissions consécutives avant transcription | protocole doc 12 | **nul, voire négatif** — piste fermée | doc 12 |
| détection automatique de la langue | 16 % de français et 28 % d'anglais mal classés | routage par le catalogue, pas par le modèle | D-langue, doc 13 |
| seuil de score supplémentaire 0,9 | balayage | −30 % de vrais (104,7 → 73,4) pour +2 points | doc 19 |
| seuil 1,0 (corroboration exigée) | balayage | 88 % de précision mais **26 appariements et 3 fréquences sur 5 éteintes** | doc 19 |
| quatre chiffres minimum | balayage | 41,1 vrais contre 94,7 pour +2 points | doc 19 |
| fenêtre ADS-B 30 s / 120 s | balayage | 91,1 / 91,9 vrais contre 94,7 à 60 s | doc 19 |
| accepter les appariements ambigus | avec / sans | refuser ne coûte rien et gagne en précision | doc 19 |
| règle « chiffres à un près » | avec / sans, 8 témoins | 8 appariements dont 1,5 vrai : **81 % de bruit** | doc 19 |
| correction lexicale sans IA (dictionnaire fermé) | plafond puis effet | `that → thai` ; avec garde-fou de langue : **+1 transmission, −4 candidats** | doc 22 |
| correction lexicale par LLM local | non testée, mais même propriété du problème | les erreurs qui coûtent sont dans les **chiffres**, intouchables sans connaître la réponse | doc 22 |
| contexte : répétition interne | plafond | 15 / 1 415 = 1,1 % ; règle en place, effet nul | doc 22 |
| contexte : valeur partagée entre voisines | plafond | **0** sur 668 transmissions sans indicatif | doc 22 |
| contexte : propagation par proximité | — | **non mesurable** avec le témoin mélangé ; refusée pour cette raison | doc 22 |
| complément de lexique d'opérateurs | avec / sans, 3 groupes | 0 appariement de plus ; hasard en hausse | Q23, Q25 |
| sortie fichier par transmission sur la station | relevés salves.py | fragmente le direct (médiane 0,1 s au lieu de 4,6 s) ; sortie continue sans effet mesuré de jour | docs 15, 20 |

## A.4 Ce qui est ouvert et déjà nommé

- **Q1** — la branche française (hors périmètre ici, mais 26 % du groupe tours).
- **Q19** — pourquoi 125,825 est la moins bien transcrite (hypothèse saturation, non vérifiée).
- **Q20** — justesse de la détection du locuteur.
- **Q23** — la source d'un lexique d'opérateurs republiable.
- **Q25** — le bonus « opérateur nommé » discrimine-t-il ? Il monte le hasard ; à mesurer comme `-fuzzy`.
- **D2** — le mode de diffusion par canal sur la station (décision du propriétaire).
- Non numéroté, doc 19 : *« one two zero heading »* — nombre avant son mot-clé, `heading` absent des rôles suffixes.
- Non numéroté, doc 22 : 47 % des transmissions ne portent aucun indicatif — **aucune astuce ne crée une information absente**.
- Non mesuré : `large-v3-turbo` n'a été comparé qu'en identification de langue et boucles (docs 10, 14), **jamais sur la métrique ADS-B contre atco2**.

## A.5 Les voies proposées par le panel, et ce qui a résisté à la réfutation

*Section remplie à la fin du panel — cinq lentilles (audio/ASR, grammaire,
appariement, enrichissement, mesure), trois propositions maximum chacune, puis un
sceptique par proposition chargé de la réfuter avec A.3.*

*(en cours)*

---

# Partie B — la version courte

*(écrite après A.5)*
