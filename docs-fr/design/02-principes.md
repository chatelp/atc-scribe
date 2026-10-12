# Principes — ce que le système de design doit tenir

*Deuxième livrable de la mission design (`MISSION.md`, §2), 12/10/2026. Dix principes, chacun
justifié par un usage de l'outil et, quand c'est possible, par un chiffre de l'inventaire
(`01-inventaire.md`). Ils ne dépendent pas du nom (D74). Ils sont l'arbitre des choix du système
(`03-systeme.md`) : une règle du système qui n'en découle pas n'a pas sa place.*

Rappel de l'usage, qui fonde tout le reste : une personne, novice en aéronautique, qui écoute tous
les jours, **souvent le soir**, sur un Mac à 1 440 px posé à 50–70 cm, et parfois sur son
téléphone par VPN ; une à sept fréquences ; demain peut-être des radioamateurs et des passionnés.
L'outil reste allumé longtemps et on le regarde par intermittence, en écoutant.

---

## 1. La nuit d'abord

**Le fond est noir, la lumière est réservée à ce qui informe.** Pas de grande surface claire, pas
de blanc pur pour le texte courant, des fonds de panneau qui se distinguent du fond par une marche
de luminance faible et une bordure, pas par une couleur.

*Pourquoi* : l'écoute se fait le soir, l'écran reste ouvert des heures ; une surface claire éblouit
et efface le reste. Le noir et le vert sont aussi le code de l'écran de contrôle, que le
propriétaire veut garder. *Mesure* : aujourd'hui fond `#0a0a0a`, panneaux `#1a1a1a`, texte
`#e0e0e0` (15:1) : la base est juste ; ce sont les petits textes sombres qui ne le sont pas.
*Conséquence* : pas de thème clair à concevoir ; le seul fond clair possible est celui de la carte
(IGN, OSM), qui garde sa palette d'avions dédiée.

## 2. Lisible à bout de bras : un plancher de 11 px

**Rien de ce qui se lit n'est sous 11 px ; quatre tailles, cinq au plus.** Une unité, un
compteur, une étiquette sont plus clairs ou plus petits que leur valeur, jamais illisibles.

*Pourquoi* : un novice doit pouvoir lire sans se pencher, à 60 cm, de nuit. *Mesure* : 45 % des
caractères de l'écran principal sont sous 11 px, 12 % à 7–8 px ; quinze tailles dans le code,
dont sept entre 7 et 11 px, qui ne hiérarchisent rien parce qu'elles ne se distinguent pas. La
page des réglages (12 et 14 px, 3 % sous AA) prouve qu'un écran dense de l'outil tient sans.
*Conséquence* : ce qui ne tient plus à 11 px se retire, se replie, ou passe dans une bulle d'aide.

## 3. La hiérarchie par des niveaux nommés, jamais par la transparence

**Trois niveaux de texte (principal, secondaire, discret), chacun une couleur pleine qui passe AA
sur chaque fond où il peut apparaître.** La transparence est réservée aux fonds et aux bordures.

*Pourquoi* : baisser l'opacité du texte principal pour faire du secondaire produit des gris qui
dépendent du fond et tombent sous le seuil. *Mesure* : dix opacités du même gris (`text-text/25` à
`/90`, 134 emplois), plus des `opacity-*` sur des blocs de texte (53 emplois d'`opacity-*` en tout) ;
c'est la cause de presque tous les échecs :
18 % des caractères de l'écran principal sous AA, 35 % sur la Timeline. Les couleurs pleines,
elles, passent toutes (6,3 à 17,4:1).

## 4. Une couleur, un sens

**Chaque teinte a un rôle et un seul.** Le vert est la voix de l'outil : la marque, ce qui est
vivant, ce qui est choisi. L'ambre appelle l'attention. Le rouge est l'urgence et la panne, rien
d'autre. Les phases de vol et les altitudes ont leurs propres palettes, qui ne réemploient pas ces
trois-là pour autre chose.

*Pourquoi* : quand une couleur dit tout, elle ne dit plus rien ; le novice apprend un code, il
doit être stable. *Mesure* : le vert porte aujourd'hui cinq sens (marque, actif, succès, pilote,
clairance), en deux verts de marque différents (`#4CAF50`, `#4ADE80`) et 17 variantes
transparentes ; 225 couleurs au total, dont 319 écrites hors du thème.

## 5. Ce qui crie est rare

**Seuls quelques événements ont droit au mouvement, au rouge ou au son** : une urgence (squawk
7500/7600/7700, MAYDAY, PAN PAN), une remise des gaz, un changement de piste, une panne de la
chaîne. Tout le reste se tait : il change en place, sans clignoter.

*Pourquoi* : on regarde l'écran par intermittence ; ce qui doit faire lever les yeux doit être la
seule chose qui bouge. *Mesure* : l'ancienne barre d'alertes, à 72 % de bruit, a été retirée le
10/10 pour cette raison ; le journal d'événements qui l'a remplacée est le bon modèle. Le reste du
mouvement est celui du ciel : les avions se déplacent, c'est l'information.
*Conséquence* : pas d'animation décorative ; `prefers-reduced-motion` respecté (absent aujourd'hui).

## 6. Jamais la couleur seule

**Toute information portée par une couleur l'est aussi par un mot, une forme ou une position.**

*Pourquoi* : 8 % des hommes voient mal le rouge et le vert ; de nuit, sur un trait fin, tout le
monde confond les teintes voisines. *Mesure* : 13 paires de couleurs confondues dans les palettes
porteuses de sens, dont 10 dans les phases : TAX et CRZ deviennent la même couleur pour un
deutéranope (ΔE 1,8), NEW et UNK se confondent même en vision normale (ΔE 5,6). Les voyants
vert / rouge ne se distinguent que par la clarté. Ce qui marche déjà : les badges de phase portent
leur code (« CRZ »), le locuteur est écrit (« ATC »).
*Conséquence* : voyants à formes différentes, code de phase toujours écrit à côté de sa couleur,
y compris sur la carte.

## 7. Le même avion partout

**Un avion a la même apparence sur la carte, dans la liste, dans la PAR, dans sa fiche et dans
le fil de transcription** : même pastille d'indicatif, même couleur et même code de phase, même
signe quand il est sélectionné, même signe quand il vient de parler.

*Pourquoi* : le geste central de l'outil est de relier ce qui se dit à ce qui vole ; l'œil doit
faire ce lien sans lire. *Mesure* : les phases sont déjà cohérentes entre les trois vues (mêmes
dix valeurs, bon point) ; la sélection, elle, s'écrit de trois façons (fond vert transparent,
ombre intérieure, bordure verte), et l'indicatif cliquable du fil n'a pas la forme de celui de la
liste.

## 8. Le monospace pour ce qui se compare, le texte pour ce qui se lit

**Indicatifs, fréquences, niveaux, caps, vitesses, heures, codes : monospace. Phrases
(transcriptions, consignes en clair, aides, messages) : police du système.** Aucune police
chargée dans les deux cas.

*Pourquoi* : le monospace aligne les colonnes de nombres sans effort et donne l'allure de l'écran
de contrôle ; mais une transcription est une phrase, et la lire en chasse fixe coûte de la place
et du confort. *Mesure* (navigateur, cinq transmissions types FR et EN) : le monospace est **1,24 à
1,32 fois plus large** que la police système à taille égale (médiane 1,26) ; une phrase en police
système à **14 px** occupe moins de largeur (477 px) que la même en monospace à 12 px (527 px). À
largeur égale, le passage gagne deux pixels de corps, ce qui règle le plancher du principe 2 sans
perdre de place. Aujourd'hui : monospace pour 100 % des caractères.
*À trancher par le propriétaire* : c'est le seul principe qui change l'allure générale.

## 9. Deux densités, pas trois

**Le bureau (1 440 px, souris, clavier, tout à l'écran) et le téléphone (375 px, doigt, un volet à
la fois). Rien entre les deux.** Le téléphone n'est pas un bureau réduit : cibles de 44 px, une
seule colonne, la fiche en plein volet, l'aide au toucher.

*Pourquoi* : ce sont les deux usages réels ; un troisième niveau double le travail pour personne.
*Mesure* : au téléphone, 35 cibles tactiles sur 41 font moins de 44 px et 17 moins de 24 px ; le
découpage en trois onglets (Map, PAR, List) du 11/10 est la bonne structure et reste.
*Conséquence* : chaque composant a au plus deux variantes de taille ; le clavier a un focus
visible au bureau (aujourd'hui aucun).

## 10. Expliquer à la demande, jamais en permanence

**Le vocabulaire d'aviation reste à l'écran, l'explication vient quand on la demande** : bulle
d'aide au survol ou à l'appui long, message vide qui dit quoi faire, jamais un paragraphe fixe qui
prend la place des données.

*Pourquoi* : le novice a besoin de comprendre, l'écran de contrôle a besoin de densité ; la bulle
sert les deux. *Mesure* : 591 éléments portent une bulle à l'écran principal (exigence du
propriétaire, tenue) ; à l'inverse, le seul texte explicatif permanent mesuré, celui de la
Timeline, est à 10 px et 3,2:1 — il serait mieux dans une bulle ou lisible.
*Conséquence* : la bulle d'aide est un composant du système à part entière, avec sa typographie
(lisible, police du système), et chaque état vide a son message.

---

## Ce que ces principes excluent

- un thème clair, un mode « jour » ;
- une troisième taille d'écran (tablette dédiée) ;
- toute animation qui n'est ni un avion, ni un événement du principe 5, ni une transition d'état
  de moins de 200 ms ;
- une couleur ajoutée sans rôle écrit dans le système ;
- un texte sous 11 px, un texte secondaire obtenu par transparence ;
- une police téléchargée (le projet ne charge rien d'un hôte extérieur depuis le 11/10).
