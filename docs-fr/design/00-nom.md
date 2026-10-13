# Le nom, et la place du français — propositions pour le propriétaire

*12/10/2026, session design. Réponse à D74 (le nom « atc-scribe » est abandonné) et à Q53 (faut-il
un positionnement « espace aérien français » ?). **Rien n'est décidé ici : Pierre choisit.** Aucun
nom n'est marqué, réservé ni acheté avant son choix.*

> **Mis à jour le 13/10.** Le premier tour (§ 3, recommandation 5/5) a été jugé trop peu original
> par le propriétaire. Un second tour dans l'histoire de l'aviation, puis un troisième dans le
> vocabulaire de la fréquence, mènent à **Clairance**, vérifié marques et domaines (§ 4). Les
> paragraphes 1 et 2 (contraintes, place du français) restent la référence.

Sources : la session « ATC-scribe communauté » (noms pris dans l'écosystème, réception probable,
vocabulaire réel, dans son dépôt privé `02-existant.md`, `03-communautes.md` § 9,
`annexes/pistes-de-nom.md`) ; une vérification de disponibilité en lecture seule le 12/10 (GitHub,
recherche web, whois et RDAP des domaines) ; `05-decisions.md` (D74, Q53).

## 1. Ce que le nom doit faire, et ne pas faire

**Doit** : être court ; se dire en français et en anglais ; dire ce que fait l'outil (entendre la
radio et comprendre ce qui s'y dit, relier ce qui se dit à ce qui vole) plutôt que ce qu'il est
techniquement ; tenir comme signe à 16 px ; ne pas être pris dans notre domaine.

**Ne doit pas** (constats de la session communauté, sources primaires) :
- **évoquer la surveillance ou l'évaluation** : *watch, monitor, track, audit, log, scan*. Le SNCTA
  a combattu SCRIBE et SLASH comme « surveillance algorithmique » (septembre 2025, préavis de
  grève) ; or des contrôleurs de métier sont présents dans VATSIM et IVAO, et les contrôleurs sont
  le premier public d'annotateurs visé ;
- **évoquer la captation ou la diffusion** : *intercept, capture, record, spy, eavesdrop, live,
  stream, broadcast, scanner*. Juridiquement, ce qui expose est la diffusion ; la DGAC a fait
  couper les flux français de LiveATC en 2022 au nom de la vie privée des contrôleurs ;
- **réemployer un mot usé ou pris** : *radar, tower, squawk, airband, sky, live, whisper*, « ATC »
  en préfixe, *readback* et *collationnement* (cœur de SLASH, et pris deux fois), *wilco*
  (WilcoATC), *copilot*, *scribe*.

**Trois messages** que la présentation doit porter dès sa première phrase, parce qu'ils répondent
aux objections déjà formulées par la DGAC et le SNCTA : **local, rien ne sort ; pour celui qui
écoute ; n'évalue personne.**

## 2. La question du français (Q53)

### Ce qui pèse

| Pour un nom et une présentation « français assumé » | Contre |
|---|---|
| Terrain vide : aucun outil ouvert ne lit la phraséologie française ; les seuls bilingues sont fermés (Scribe de la DSNA, Enhanced Radar) | Le code est générique : n'importe quel flux audio par l'API des sources, n'importe quel tar1090 ; un nom français ferme la porte en apparence |
| Les publics français visés (IVAO France, qui exige le français ; aéroclubs ; BIA ; FCL.055) n'ont aucun outil sur trafic réel : « enfin un outil pour nous » | Les briques auxquelles on se branche (readsb, tar1090, sdr-enthusiasts, adsb.im) vivent en anglais, et les contributeurs ADS-B français n'ont pas de communauté propre |
| Une identité claire, la seconde lecture française est l'apport technique majeur (mission, apport n° 2) | **« Espace aérien français » se lit « écoute les contrôleurs français »** : exactement le terrain de la DGAC contre LiveATC et du SNCTA contre SCRIBE |
| | Le dépôt public est trouvé en anglais (recherche GitHub, mots-clés) |

*Mesuré* : une recherche GitHub sur « atc transcription » rend 33 dépôts, « atc whisper » 50 ;
aucun ne traite le français d'après la session communauté. Je n'ai pas pu compter les dépôts qui
mentionnent le français (limite de débit de l'API GitHub atteinte) : c'est le seul chiffre manquant.

### Recommandation

**Porter le français comme une capacité, pas comme un territoire.** La session communauté arrive
à la même conclusion par un autre chemin.

- **Nom** : un mot qui existe dans les deux langues, ou une expression qui a son double exact dans
  l'autre langue. Le bilinguisme est alors dans le nom lui-même, sans nommer la France.
- **Page GitHub en anglais**, avec un **README français complet, à égalité** (pas un résumé).
  Première phrase des deux : les trois messages.
- **Description** (proposition, 132 caractères) : *« Hear the airband you receive, read it in
  English and French, see which aircraft is speaking. Local; for the listener; judges no one. »*
- **Mots-clés** : `adsb`, `airband`, `atc`, `speech-to-text`, `bilingual`, `french`, `rtl-sdr`,
  `tar1090`, `self-hosted`, `aviation`. **Pas** : `monitoring`, `live`, `stream`, `scanner`,
  `surveillance`.
- **Ce qui dit « français »** : la phrase « reads French ATC phraseology as well as English » en
  tête des capacités ; la vue PAR et les altitudes QNH sous la TA décrites comme des réglages,
  avec la France comme premier pays servi.

Le positionnement « français assumé » reste possible avec les mêmes noms : il suffit de mettre le
README français en premier et de choisir la forme française du nom (voir les variantes). Les deux
questions se tranchent donc ensemble, sur le même tableau.

## 3. Les propositions

Disponibilité relevée le 12/10, en lecture seule. **GitHub** : total des dépôts dont le nom
contient le mot (gonflé par les noms composés), et collisions dans notre domaine (aviation, ATC,
radio, SDR, ADS-B, transcription). Le compte GitHub du même nom compte peu : le dépôt vivrait sous
`chatelp/`. **Domaines** : whois (.org, .io, .fr) et RDAP (.app, .dev) ; « libre » veut dire
« pas de réponse d'enregistrement », pas « achetable » (rien n'a été vérifié chez un registrar).
**Non vérifié** : marques déposées (INPI, EUIPO, USPTO), .com, npm, PyPI, Docker Hub, magasins
d'applications, réseaux sociaux.

### A. **5/5** — dit « cinq sur cinq » en français, « five by five » en anglais *(recommandé)*

- **Sens** : « je vous reçois cinq sur cinq » : lisibilité 5 sur l'échelle radio, compris
  parfaitement. C'est la promesse de l'outil : rendre la fréquence lisible. L'expression est
  courante en français hors aviation (tout le monde la connaît) ; *five by five* est l'expression
  radio anglaise équivalente, connue des radioamateurs (leur « 59 ») et des pilotes.
- **Bilinguisme** : le même signe écrit, lu dans la langue de chacun. Aucun pays nommé.
- **Prononciation** : FR /sɛ̃k syʁ sɛ̃k/ ; EN /faɪv baɪ faɪv/. Chaque lecteur dit la sienne.
- **Forme écrite** : la marque s'écrit `5/5` ; le dépôt et les fichiers ont besoin d'un nom sans
  barre : `five-by-five` (positionnement générique) ou `cinq-sur-cinq` (français assumé).
- **Disponibilité** :
  - GitHub : `cinqsurcinq` 0 dépôt, `cinq-sur-cinq` 8 ; `five-by-five` 337, `fivebyfive` 50 ;
    **aucune collision dans notre domaine** (un seul dépôt `5by5-AirCraftAPI`, 1 étoile, vide) ;
  - domaines `cinq-sur-cinq` et `cinqsurcinq` : .org, .app, .dev, .io libres, .fr pris ;
    `five-by-five` : .app, .dev, .io, .fr libres, .org pris ;
  - `5by5` est à éviter : tous les domaines pris, réseau de podcasts du même nom.
- **Signe** : le chiffre lui-même (« 5/5 » en monospace, trois caractères, tient à 16 px) ou
  l'échelle de lisibilité (cinq barreaux, le cinquième allumé). Deux états de la barre de menus
  naturels : échelle pleine en marche, éteinte à l'arrêt.
- **Risque** : expression banale, donc difficile à protéger comme marque ; « five by five » a
  d'autres usages culturels, sans rapport avec la radio.

### B. **Entendu** *(français assumé)*

- **Sens** : à la fois « heard », « understood » et « agreed » ; « bien entendu ». Dit ce que fait
  l'outil (il entend et fait comprendre), sans rien de la surveillance.
- **Prononciation** : FR /ɑ̃.tɑ̃.dy/ ; EN approximatif « on-ton-dew », pas naturel.
- **Disponibilité** : GitHub 15 dépôts, aucune collision (un outil d'apprentissage des langues,
  `deja-entendu`, 2 étoiles) ; .dev et .io libres, .org, .app, .fr pris.
- **Signe** : les guillemets français « », signe typographique propre au français, qui dit
  « parole rapportée » ; tiennent à 16 px.
- **Risque** : se ferme aux anglophones à l'oral ; ne dit rien de l'avion.

### C. **En clair**

- **Sens** : « une consigne en clair » (l'outil traduit la phraséologie en phrase simple : c'est une
  de ses fonctions, audit 3.5). Existe tel quel en anglais (*en clair* : en langage clair).
- **Prononciation** : FR /ɑ̃ klɛʁ/ ; EN /ɒn ˈklɛər/ (le mot est dans les dictionnaires anglais).
- **Disponibilité** : `en-clair` 65 dépôts, `enclair` 8, aucune collision dans notre domaine ;
  `en-clair` .org, .app, .dev, .io libres, .fr pris. `clair` seul est **inutilisable** (Quay Clair,
  11 000 étoiles ; tous domaines pris).
- **Signe** : une ouverture, des crochets qui s'ouvrent ; moins immédiat.
- **Risque, réel** : en anglais, *en clair* s'emploie surtout dans le renseignement d'origine
  électromagnétique, pour un message intercepté non chiffré. Pour un lecteur anglophone averti,
  c'est exactement la connotation à éviter. Je la signale ; elle me fait la classer après A.

### D. **Verticale** *(français assumé)*

- **Sens** : « à la verticale du terrain » : ce qui passe au-dessus de la station. Image juste de
  l'outil (ce qu'on reçoit d'ici), mot de pilote.
- **Prononciation** : FR /vɛʁ.ti.kal/ ; EN se lit *vertical*, compris.
- **Disponibilité** : GitHub 132 dépôts, aucune collision ; .app libre, le reste pris.
- **Signe** : un trait vertical au-dessus d'un point. Très fort à 16 px, le meilleur des cinq en
  signe pur.
- **Risque** : *vertical* en anglais évoque le « marché vertical » ; ne dit rien de la parole.

### E. **Lisible** *(français assumé)*

- **Sens** : la lisibilité de l'échelle radio (« lisibilité cinq »), et le but de l'outil.
- **Prononciation** : FR /li.zibl/ ; EN difficile.
- **Disponibilité** : GitHub 39 dépôts, aucune collision trouvée (recoupement partiel, limite de
  débit) ; .dev libre, le reste pris.
- **Signe** : faible ; reprend l'échelle de A sans son évidence.
- **Risque** : fait double emploi avec A, en moins bien.

### Écartés à la vérification

| Nom | Pourquoi |
|---|---|
| Liaison | Le mot existe dans les deux langues et dit le lien radio ; mais 661 dépôts, **tous les domaines pris**, nom d'éditeurs connus |
| Accord | 36 346 dépôts, tous domaines pris, Honda Accord, Accord.NET, homophone d'Accor |
| Party line | Terme de pilote (entendre les autres sur la fréquence), mais **collision radio** : `RFnexus/partyline`, application vocale sur Reticulum (53 étoiles) |
| Clair | Quay Clair (scanner de conteneurs, 11 000 étoiles) |
| Roger, Contact, Copy | Génériques, impossibles à trouver |

### Le tableau du premier tour (12/10, dépassé)

| | Générique, français comme capacité | Français assumé |
|---|---|---|
| **A. 5/5** | `five-by-five`, README anglais en premier | `cinq-sur-cinq`, README français en premier |
| **B. Entendu** | — (ne se dit pas en anglais) | `entendu` |
| **C. En clair** | `en-clair` (risque de connotation en anglais) | `en-clair` |
| **D. Verticale** | — | `verticale` |
| **E. Lisible** | — | `lisible` |

La recommandation de ce tour était A (5/5). Le propriétaire, le 12/10 au soir : « ça manque
franchement d'originalité » ; chercher dans l'histoire de l'aviation, française en particulier.

## 4. Second et troisième tours (12 et 13/10) : Clairance

### Ce que le propriétaire a dit

Sur une liste prise dans l'histoire (Gonio, la radiogoniométrie de l'Aéropostale ; Demoiselle,
l'avion de Santos-Dumont aux plans donnés ; Éole ; Alouette ; Chandelle ; Timbre ; Identifié ;
Reçu ; Galène, le poste à galène des années 1920), **Galène** avait sa préférence, sans le
convaincre tout à fait. Sur une liste de mots à double sens (Aplomb, Portée, Volubile, Envolée) :
« un nom qui fasse sérieux, Volubile ne va pas du tout, un rapport direct avec l'aviation, moins
cryptique qu'atc-scribe ». Deux leçons : un mot de la fréquence elle-même, et aucun jeu de mots.

### Écartés à la vérification (13/10)

| Nom | Pourquoi |
|---|---|
| Galène | **pris** : `jech/galene`, serveur de visioconférence libre et français, 1 418 étoiles, avec un module `galene-stt` de transcription de la parole ; .org, .app, .dev, .io, .fr enregistrés |
| Indicatif | bibliothèque Rust `console-rs/indicatif`, 5 224 étoiles |
| Survol | extension de navigateur `mdolr/survol`, 164 étoiles ; domaines pris ; « sur- » voisin de surveillance |
| Vol de nuit | la vraie histoire de l'outil (Rivière écoutant les avions dans la nuit), mais parfum Guerlain depuis 1933 et succession Saint-Exupéry |
| Pionniers (Mermoz, Saint-Ex, Latécoère, Dassault) | successions et marques actives ; un nom propre ne dit pas ce que fait l'outil |
| Phonie | le vrai mot de la radio aéronautique française ; se lit *phony* en anglais |

### Le troisième tour : les mots de la fréquence

| Nom | Ce qu'il dit | GitHub (nom contient) | Domaines | Verdict |
|---|---|---|---|---|
| **Clairance** | l'autorisation que donne le contrôleur, orthographe du contrôle français ; porte « clair » | 4, tous médicaux | .org .app .dev libres ; .io .fr .com pris | **recommandé** |
| Phraséo | la phraséologie, langue codée de la radio, nom courant chez pilotes et contrôleurs (le manuel de la DSNA s'appelle `phraseo`) | 171, surtout *phraseology* | .org .io .dev libres | bon second ; « salle de cours » |
| Approche | la phase où le ciel parle le plus, le nom des fréquences écoutées | 872, au sens de « méthode » | probablement pris | compris de tous, impossible à posséder |
| Bande aéro | le nom que les écouteurs donnent à la bande VHF aviation | 0 | .org .io .fr libres | descriptif, nom de catégorie |
| Portance | la force qui porte l'aile | 3 | .dev libre | sérieux, mais ne dit rien de la radio |

### Clairance, vérifié le 13/10

- **Sens.** Le mot central de la radio aéronautique : « clairance de décollage », « clairance ILS »,
  ce que l'outil met en clair sous chaque transmission (« Cleared ILS runway 26L »). Compris d'un
  anglophone (*clearance*) et de tout spectateur de film. Autres sens, hors aviation : la clairance
  rénale (médecine) ; en anglais, le dédouanement, l'habilitation, les soldes. Pour la sensibilité
  du SNCTA : la clairance est l'acte du contrôleur, pas son évaluation.
- **Prononciation.** FR /klɛ.ʁɑ̃s/ ; EN *clearance*.
- **Code.** GitHub : 4 dépôts, tous sur la clairance de la créatinine ; npm, PyPI, crates.io,
  Docker Hub : aucun paquet ; App Store : aucune application de ce nom (seulement des calculateurs
  médicaux et des applications de soldes « clearance »). Google Play non vérifié.
- **Domaines.** `clairance.org`, `.app`, `.dev` libres ; `.io` et `.fr` pris ; `.com` pris depuis
  2011 (OVH) par un cabinet lyonnais de cession d'entreprise, site actif. La paire naturelle est
  `.org` + `.app`.
- **Marques** (TMview, 13/10, qui agrège INPI, EUIPO, OMPI, USPTO et les offices nationaux) :
  11 marques contenant « clairance », toutes déposées en France, en Suisse ou en Tunisie ; **aucune
  à l'EUIPO**, aucune aux États-Unis. **Aucune marque « CLAIRANCE » seule n'est active** : les
  trois qui existaient sont expirées (Suisse 1990, cl. 3 ; INPI 1991, cl. 3 et 42 ; INPI 1997,
  cl. 11). Actives : « CLAIRANCE Avocats » (INPI 2021, cl. 16, 41, 45 : un cabinet, hors
  logiciel) et « Eclairance » / « éclairance » (INPI 2020 et 2022, cl. 35, 36, 37, 41, 42, 45) : un
  autre mot, à une lettre près, qui couvre la classe 42 (services logiciels). `data.inpi.fr` rend
  0 marque sur le mot, son moteur étant moins complet que TMview. La recherche exacte du mot
  anglais « clearance » a échoué sur TMview (erreur de validation) : non vérifiée.
- **Lecture.** Pour un logiciel libre qui ne vend rien, rien ne bloque l'usage du mot. Si un dépôt
  de marque était voulu un jour (cl. 9 et 42), « éclairance » serait l'antériorité à faire examiner
  par un conseil.
- **Forme.** Marque « Clairance », dépôt `chatelp/clairance`, signe : une piste et une coche,
  « autorisé ». Positionnement générique (§ 2) : page GitHub en anglais, README français complet à
  égalité, le français comme capacité.

**Ma recommandation : Clairance.** Sérieux, aviation sans détour, compréhensible du premier coup,
et il dit ce que fait l'outil : rendre la clairance claire.

**Reste à faire après le choix** : la lecture du brevet US12277862B1 (Q53) par quelqu'un de
compétent, avant toute diffusion hors de France ; et, si le propriétaire veut les domaines,
`clairance.org` et `clairance.app` à prendre chez un registrar (rien n'a été réservé).

## 5. Ce qui suit le choix

La marque se conçoit comme un système qui ne dépend pas du mot : la typographie du nom (monospace,
principe 8), un signe à 16 px, l'icône de la barre de menus à deux états, la page de connexion et
l'accueil. Je la dessine seulement après le choix. Le renommage du dépôt, du lanceur, du dossier
de configuration, du titre de page et des documents se fait ensuite en une fois (D74), par la
session atc-scribe.
