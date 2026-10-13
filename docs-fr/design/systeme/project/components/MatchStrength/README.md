# MatchStrength

Match strength shows how sure the tool is that a transmission was matched to the right aircraft.

**Markup.** `<span class="cl-strength cl-strength--3"><i></i><i></i><i></i></span>`, `--1` to `--3`; `title` says it in words ("Strong clues").

**Rules.** The number of filled bars says the strength in `color-text-secondary`; only the weak one turns `color-status-warn`, because it is a reason to doubt. The chip of a weak match reads "probably …" (`Chips`, `is-guess`).

**Data.** `match_score` and `match_reason` of `/api/v1/transcriptions`; an unmatched transmission keeps its reason (`unmatched: …`).
