# `docs-fr/` — le dossier de chantier

Ce dossier est **la documentation de travail du fork `co-atc-local`**, en français.
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
| `19-appariement-en-ligne.md` | la mise en service de l'appariement : point de greffe, réglages mesurés, limites |
| `CLAUDE-amont.md` | copie des instructions amont, conservées telles quelles |

`05-decisions.md` se tient à jour au fil de l'eau. C'est la seule consigne non
négociable de ce dossier.
