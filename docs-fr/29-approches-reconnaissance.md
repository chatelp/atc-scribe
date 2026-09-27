# Améliorer la transcription : le modèle de langage et le son qui lui arrive

*Document de référence, état au 27 septembre 2026. Il se lit sans jargon ; chaque ligne renvoie au
document qui détaille la mesure. À tenir à jour à chaque résultat.*

*Depuis le 27/09, ce document ne couvre plus que **le modèle, la chaîne de transcription et la
réception**. Les règles qui relient ensuite le texte à un avion, avec leur réglage dans co-atc, sont
dans **`30-association-regles-et-reglages.md`** (l'ancienne section B de ce tableau).*

## Ce qu'on cherche, et comment on juge

**Le but** : que co-atc relie chaque transmission au bon avion — son indicatif — et qu'il
**n'invente pas**. Un indicatif inventé qui tombe sur un avion réel est pire qu'une absence : on
ne le voit pas.

**Deux juges** :
- **l'ADS-B** : une transmission est « juste » si l'indicatif compris correspond à un avion
  réellement dans le ciel à ce moment. On retire la part due au hasard en refaisant le même
  calcul avec un ciel pris à un autre moment (le « contrôle par ciel mélangé ») ;
- **l'oreille du propriétaire** : 55 transmissions transcrites à la main — mais peu en anglais, et
  presque toutes partielles.

**Le point de départ** (Q30) : sur 1 332 transmissions du 15/09, **environ 8 % reliées à un
avion juste**. Les pertes viennent surtout des **chiffres mal entendus** et de transmissions qui
ne portent aucun indicatif (47 %).

## Le tableau

✅ adopté · ❌ écarté · ⏳ en cours · 🔲 pas encore testé

### A. Le modèle et la chaîne de transcription

| | Approche | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 1 | Un modèle anglais spécialisé contrôle aérien, le plus gros | 94 avions justes contre 63 pour un modèle moyen spécialisé | ✅ (D12) |
| 2 | Tout faire tourner sur le Mac, sans service extérieur | 0,7 s par transmission | ✅ (D3, D10) |
| 3 | Une seconde lecture par un modèle français quand le texte contient du français | +8,5 % d'avions justes ; elle se déclenche sur 12,5 % des transmissions | ✅ (D24) |
| 4 | Un détecteur de voix pour écarter le bruit | la nuit, jusqu'à 98 % des ouvertures du squelch sont sans parole | ✅ (D27) |
| 5 | Donner au modèle un texte d'amorce | 7 fois plus de délires | ❌ (doc 10) |
| 6 | Donner en amorce les avions visibles à l'ADS-B | aucun effet net sur un petit échantillon | ❌ (D46) |
| 7 | Coller les transmissions consécutives avant de transcrire | nul ou négatif | ❌ (doc 12) |
| 8 | Laisser le modèle deviner la langue | 16 à 28 % d'erreurs ; remplacé par la langue connue de chaque fréquence | ❌ (doc 13) |
| 9 | Les réglages internes du modèle contre l'invention | aucun ne sépare l'invention de la bonne transcription | ❌ (D45) |
| 9 bis | Ne donner au modèle que la parole | l'invention baisse avec la marge (presque divisée par deux à 1 s) ; les avions justes ne bougent pas au-delà du bruit, mais la fréquence faible 125,825 en perd à toutes les marges | ⏳ pas adopté en l'état (Q41) |
| 9 ter | **L'aiguillage vers le modèle français** : aujourd'hui, la seconde lecture ne se déclenche que si le texte *anglais* contient un mot français repère (« bonjour », « niveau »…) | 27/09, 126,425 : le propriétaire entend « Air France 511 » dans des échanges en français ; forcé en anglais, le modèle **anglicise** (« Air France Five One Air we conceded report three zero »), aucun repère, pas de seconde lecture. La détection de langue de Whisper, avec notre modèle affiné sur l'ATC anglais, répond « anglais » sur ces mêmes passages | 🔲 aiguiller autrement : détection de langue par un modèle multilingue non affiné, ou mots repères plus larges, mesuré sur les passages français connus |

### B. Du texte à l'avion

→ **déplacé dans `30-association-regles-et-reglages.md`** (règles, réglages du panneau, mesures).

### C. La réception, à la station

| | Approche | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 19 | **Recevoir chaque fréquence séparément au lieu du mélange** | **4 fois plus d'avions justes** (235 contre 60), précision égale — le gain le plus fort mesuré à ce jour | ✅ flux séparés en service à la station depuis le 26/09 ; co-atc les suit (D57) — séances du 26/09 et nuit du 26 au 27 ; reste le lancement de la production |
| 20 | Comprendre le hachage | le squelch se ferme par instants au milieu des transmissions ; **la position de la fréquence dans la bande n'y est pour rien** (compteurs de la station, 26/09) ; il varie d'un facteur 10 d'un quart d'heure à l'autre ; **le contrôleur, au sol, hache presque autant que les pilotes** (27/09 : 56 % contre 63 % des transmissions) : la cause est à la réception, pas à la distance de l'avion | ⏳ séance du 27/09 en cours d'analyse (Q47) |
| 20 bis | Deux fréquences qui reçoivent la même chose | le contrôle couple ses secteurs le soir et la nuit (approches de De Gaulle : 65 à 100 % des transmissions en double) : écho à l'écoute, transcriptions en double | ✅ compris (Q48) : n'écouter qu'une des deux |
| 21 | Chasser le parasite de nuit | toujours là ; le bloc de la Freebox est innocenté | ⏳ coupable inconnu (Q44) |
| 22 | Baisser le gain de la réception | ça écrête, mais baisser le gain n'est pas le remède | ❌ (D42, D43) |
| 23 | Nettoyer le son par des filtres classiques | pas encore mesuré ; la station publie déjà un flux filtré | 🔲 (Q42) |

### D. Entraîner le modèle sur le bruit de la station (plan 27)

| | Étape | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 24 | Peut-on entraîner sur ce Mac ? | oui : ~5 s par pas, 9 Go de mémoire ; reconversion pour co-atc réussie | ✅ |
| 25 | Faire sonner des enregistrements propres comme la station | l'imitation reproduit la station sur tout ce qui se mesure | ✅ |
| 26 | Mini-essai 1 : un seul corpus, le modèle entier | invente **4 à 6 fois moins** sur du bruit, mais **deux fois moins d'avions justes** : il a appris le vocabulaire du corpus (« Rhein », « Lufthansa ») et perdu celui de Paris | ❌ en l'état |
| 27 | Mini-essai 2 : un mélange de trois corpus | 191 avions justes (essai 1 : 104 ; production : 236) ; invente toujours moins sur le bruit ; la fréquence faible 125,825 perd encore la moitié de ses avions | ❌ en l'état, mais la bonne direction |
| 28 | Mini-essai 3 : le mélange, en n'entraînant que « l'oreille » (pas la rédaction) | 158 avions justes, moins bien que l'essai 2 sur les quatre fréquences ; plus lent (textes à rallonge) | ❌ |
| 28 bis | Ce que les essais 2 et 3 ont en commun | sur les fréquences faibles, ils rendent **vide un tiers à la moitié des morceaux qui contiennent de la parole** (8 % en production) : la leçon « se taire sur le bruit » déborde sur la parole faible | à corriger au prochain essai |
| 28 ter | L'invention sur le bruit, là où elle compte | la baisse mesurée porte sur des ouvertures que co-atc n'envoie déjà pas au modèle ; sur les morceaux qu'il transcrit vraiment, **pas de gain** (texte trop long : 10,7 % production, 12,1 % essai 2) | ❌ l'acquis apparent ne tient pas |

### E. Qui parle, contrôleur ou pilote

| | Approche | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 29 | Deviner à partir des mots | pas fiable, et aucune référence pour le vérifier ; une règle plus prudente n'a pas fait mieux | ⏳ gardé faute de mieux (Q39) |
| 30 | Reconnaître le contrôleur à sa voix | pas encore essayé ; demande le son de chaque fréquence séparé | 🔲 |

## Ce qui reste à tester, par ordre d'intérêt

1. **L'aiguillage vers le modèle français** (9 ter) : du français « anglicisé » par le modèle anglais
   n'ouvre pas la seconde lecture. À mesurer sur des passages français connus avant de changer la
   porte.
2. **Un quatrième essai d'entraînement, s'il vaut la peine** : repartir de l'essai 2 (le modèle
   entier), avec **beaucoup moins de clips de silence** pour qu'il ne se taise plus sur la parole
   faible, un apprentissage plus doux, puis le vocabulaire de Paris (vos indicatifs par synthèse
   vocale). Aucun des trois essais ne bat la production, et leur baisse d'invention sur le bruit
   est déjà obtenue par le détecteur de voix : c'est un chantier de fond, pas un gain rapide.
3. **Un jeu de test fiable** : une centaine de transmissions des fréquences visées, transcrites en
   entier, avec l'aide de l'outil d'annotation. Sans lui, on ne juge que l'indicatif.
4. **Le squelch des deux fréquences faibles**, essayé avec retour automatique et jugé sur les
   compteurs de la station.
5. **Ne donner au modèle que la parole** : mesuré le 25/09 (Q41) — l'invention baisse nettement,
   les avions justes ne gagnent rien de mesurable, et la fréquence faible en perd. À reprendre
   avec un détecteur de voix plus sensible pour ce tri, jugé sur un autre jour.
6. **Garder les deux modèles en mémoire** : l'attente derrière le modèle français fait perdre des
   transmissions aux heures chargées (6 le matin du 25/09).
7. **Le nettoyage du son par filtres**, mesuré hors ligne avant tout réglage de la station (Q42).
8. **Le locuteur par la voix**.
9. **Le modèle « turbo »**, plus rapide, jamais comparé sur l'ADS-B.

## Ce qu'on a appris en chemin

- **Juger sur la vraie station.** L'imitation a flatté le premier mini-essai ; seule la station a
  montré qu'il perdait les indicatifs de Paris.
- **Les erreurs qui coûtent sont dans les chiffres et dans l'invention**, pas dans l'orthographe.
- **Un système qui répond toujours est pire qu'un qui s'abstient** : c'est pourquoi chaque essai
  se juge avec le contrôle par ciel mélangé et sur sa précision, pas seulement sur le volume.
- **Le mélange de quatre fréquences dans un seul flux coûte trois avions justes sur quatre.**
