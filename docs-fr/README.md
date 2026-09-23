# `docs-fr/` — le dossier de chantier

Ce dossier est **la documentation de travail du fork `atc-scribe`**, en français.
Ce n'est pas une traduction de `docs/` : `docs/` appartient au projet amont et reste
tel quel, y compris `LOCAL-STT.md`, qui nous sert de référence.

Convention du fork : **code, commentaires et messages de commit en anglais ;
documents de chantier et échanges avec le propriétaire de la station en français.**

| Fichier | Contenu |
|---|---|
| `00-mission.md` | ce qu'on veut faire et pourquoi, le périmètre, le critère de réussite |
| `01-station.md` | l'installation existante : machine, réseau, chaînes RF, groupes de fréquences |
| `02-co-atc-amont.md` | le projet amont, **vérifié dans le code** |
| `03-transcription.md` | le cœur technique : bilinguisme, modèles candidats, moteurs, détection de voix |
| `04-corpus.md` | les 4 642 enregistrements réels disponibles et le protocole d'évaluation |
| `05-decisions.md` | **décisions prises et questions ouvertes — la mémoire du chantier** |
| `06-catalogue.csv` | les fréquences mesurées : niveau, écart crête-médiane, langue, à transcrire ou non |
| `07-carte-openai.md` | où l'amont appelle OpenAI, avec quels formats — le plan du remplacement |
| `08-premiere-mesure.md` | premières mesures de transcription locale sur le corpus réel |
| `09-jeu-de-test.md` | construction du jeu d'évaluation et protocole d'annotation |
| `10-mesure-amorces.md` | ce que coûte l'amorce de transcription : dégénérescence ×7 |
| `11-demande-station.md` | ce qui a été demandé à l'agent de la station, et pourquoi |
| `12-concatenation.md` | concaténer les transmissions consécutives : mesure et verdict |
| `13-strategie-bilingue.md` | la stratégie bilingue : routage par langue, modèles candidats |
| `14-mesure-modeles-francais.md` | comparatif des Whisper francisés sur le corpus |
| `15-chaine-locale.md` | la chaîne locale de bout en bout : sidecar, segmenteur, contrat |
| `16-grammaire-et-125933.md` | la grammaire de phraséologie, et l'identification de 125,933 |
| `17-appariement.md` | l'appariement des indicatifs contre l'ADS-B : méthode et témoin |
| `18-nuit-du-15.md` | la nuit de mesure : modèle anglais tranché, étagement de la TMA |
| `19-appariement-en-ligne.md` | la mise en service de l'appariement : point de greffe, réglages mesurés, limites |
| `20-captation-nuit.md` | **compte rendu de l'agent station** : captation nocturne 132-133, 4 h, 7 canaux |
| `21-nuit-132-133.md` | la nuit du 15 : 88 % de déclenchements sans parole, le peigne à 100 Hz |
| `22-correction-lexicale.md` | corriger le texte sans IA : mesuré, gain nul, **abandonné** |
| `23-voies-restantes.md` | **où on en est, et ce qu'il reste** : état mesuré, tout ce qui a été éliminé, le panel de relecture, la version courte à la fin |
| `24-identification-amas.md` | **les sept fréquences de l'amas identifiées canal par canal** : Paris arrivée, Brest, cinq croisières |
| `25-atis-131025-au-gain.md` | l'ATIS de Saint-Cyr au gain : il ne sature pas, et ne porte pas de météo à 21 h (Q32) |
| `26-ce-que-co-atc-interprete.md` | **référence** : ce que co-atc comprend et affiche, par la voix et par l'ADS-B, vérifié dans le code — état au 23/09 |
| `CLAUDE-amont.md` | copie des instructions amont, conservées telles quelles |

> Le document 20 arrive numéroté 15 par l'agent de la station, qui ne pouvait pas
> savoir que `15-chaine-locale.md` existait déjà. Renuméroté ici, contenu intact.

> Le dépôt s'appelait `co-atc-local` jusqu'au 16 septembre (D20). `11-demande-station.md`
> et `20-captation-nuit.md` gardent l'ancien nom : c'est de la correspondance datée entre
> agents, et on ne réécrit pas une lettre après coup.

`05-decisions.md` se tient à jour au fil de l'eau. C'est la seule consigne non
négociable de ce dossier.
