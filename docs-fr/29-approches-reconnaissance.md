# Améliorer la reconnaissance : toutes les approches, où on en est

*Document de référence, état au 25 septembre 2026, 20 h. Il se lit sans jargon ; chaque ligne
renvoie au document qui détaille la mesure. À tenir à jour à chaque résultat.*

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

### B. Du texte à l'avion

| | Approche | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 10 | Des règles de phraséologie à la place d'une IA en ligne | 66 % des transmissions livrent une valeur (niveau, cap, piste…) | ✅ (D14) |
| 11 | Réglages de l'association : 3 chiffres, 60 s, refuser l'ambigu | fixés par balayage ; 72 % de précision | ✅ (doc 19) |
| 12 | Corriger les mots avec un dictionnaire | gain nul | ❌ (doc 22) |
| 13 | Compléter la liste des compagnies | 0 avion juste de plus | ❌ (Q23) |
| 14 | Une table des indicatifs radio qui trompent (« France Soleil », « Bee Line »…) | mécanisme prouvé ; effet sur le résultat final **non mesuré** | ✅ (D47) |
| 15 | Accepter un chiffre d'écart | 81 % de bruit | ❌ (doc 19) |
| 16 | Accepter deux chiffres au lieu de trois | +28 % d'avions justes, mais la précision tombe de 72 à 52 % | ❌ (Q30) |
| 17 | Lire correctement les mots accentués | corrigé ; aucun effet sur l'association | ✅ (Q38) |
| 18 | Une petite IA pour l'association | évaluée sur le papier le 23/09 ; protocole prêt, avec contrôle par ciel mélangé | 🔲 |

### C. La réception, à la station

| | Approche | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 19 | **Recevoir chaque fréquence séparément au lieu du mélange** | **4 fois plus d'avions justes** (235 contre 60), précision égale — le gain le plus fort mesuré à ce jour | ⏳ demandé à la station (Q45) |
| 20 | Comprendre le hachage | c'est le squelch, sur **deux** fréquences faibles, par épisodes ; pas de coût mesurable sur l'association | ⏳ essai de réglage à faire (Q37, doc 28) |
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

### E. Qui parle, contrôleur ou pilote

| | Approche | Ce qui a été mesuré | Verdict |
|---|---|---|---|
| 29 | Deviner à partir des mots | pas fiable, et aucune référence pour le vérifier ; une règle plus prudente n'a pas fait mieux | ⏳ gardé faute de mieux (Q39) |
| 30 | Reconnaître le contrôleur à sa voix | pas encore essayé ; demande le son de chaque fréquence séparé | 🔲 |

## Ce qui reste à tester, par ordre d'intérêt

1. **La réception séparée par fréquence, en permanence** (Q45) : le gain est déjà mesuré, reste à
   l'installer à la station et à brancher co-atc dessus.
2. **Un quatrième essai d'entraînement, s'il vaut la peine** : repartir de l'essai 2 (le modèle
   entier), avec **beaucoup moins de clips de silence** pour qu'il ne se taise plus sur la parole
   faible, un apprentissage plus doux, puis le vocabulaire de Paris (vos indicatifs par synthèse
   vocale). Aucun des trois essais ne bat la production : c'est un chantier de fond, pas un gain
   rapide.
3. **Un jeu de test fiable** : une centaine de transmissions des fréquences visées, transcrites en
   entier, avec l'aide de l'outil d'annotation. Sans lui, on ne juge que l'indicatif.
4. **Le squelch des deux fréquences faibles**, essayé avec retour automatique et jugé sur les
   compteurs de la station.
5. **Ne donner au modèle que la parole** : aujourd'hui, un morceau accepté part au modèle avec son
   bruit, et c'est là qu'il invente (Q41).
6. **Garder les deux modèles en mémoire** : l'attente derrière le modèle français fait perdre des
   transmissions aux heures chargées (6 le matin du 25/09).
7. **Le nettoyage du son par filtres**, mesuré hors ligne avant tout réglage de la station (Q42).
8. **Une petite IA pour l'association**, jugée avec le contrôle par ciel mélangé.
9. **Le locuteur par la voix**.
10. **Le modèle « turbo »**, plus rapide, jamais comparé sur l'ADS-B.

## Ce qu'on a appris en chemin

- **Juger sur la vraie station.** L'imitation a flatté le premier mini-essai ; seule la station a
  montré qu'il perdait les indicatifs de Paris.
- **Les erreurs qui coûtent sont dans les chiffres et dans l'invention**, pas dans l'orthographe.
- **Un système qui répond toujours est pire qu'un qui s'abstient** : c'est pourquoi chaque essai
  se juge avec le contrôle par ciel mélangé et sur sa précision, pas seulement sur le volume.
- **Le mélange de quatre fréquences dans un seul flux coûte trois avions justes sur quatre.**
