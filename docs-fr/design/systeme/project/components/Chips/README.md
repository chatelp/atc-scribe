# Chips

Chips name what an exchange is about: the aircraft, the instruction, the speaker.

**Callsign** `cl-chip cl-chip--callsign`, with its `cl-phase cl-phase--small` inside: opens the aircraft. States: `is-selected` (accent outline), `is-away` (dashed, `color-text-tertiary`: heard but not seen, or a guess too uncertain to name, written "BAW4LK?"), `is-guess` (warn outline, "probably TVF61WB").

**Instruction** `cl-chip cl-chip--instr`: the instruction in plain English, verb first, numbers as the brand writes them ("Descend to FL 070", "Contact 128.750"). A clearance adds `cl-chip--clearance` (✓ and weight 600). No colour: green no longer means clearance.

**Speaker** `cl-chip cl-chip--atc` reads "ATC"; the pilot is the callsign chip; an unknown speaker is "?" in `color-text-tertiary`; an unknown speaker who named an aircraft shows the callsign chip alone, without "?" or arrow. An arrow (→) goes from speaker to addressee. The callsign chip is drawn 20 px high and clicked on 24.

**Data.** `speaker_type`, `callsign`, `plain[].text` (and `text_fr`) of `/api/v1/transcriptions`.
