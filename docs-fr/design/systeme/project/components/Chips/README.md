# Chips

Chips name what an exchange is about: the aircraft, the instruction, the speaker.

**Callsign** `cl-chip cl-chip--callsign`, with its `cl-phase cl-phase--small` inside: opens the aircraft. States: `is-selected` (accent outline), `is-away` (dashed, `color-text-tertiary`: heard but not seen, or a guess too uncertain to name, written "BAW4LK?"), `is-guess` (warn outline, "probably TVF61WB").

**Instruction** `cl-chip cl-chip--instr`: the instruction in plain English, verb first, numbers as the brand writes them ("Descend to FL 070", "Contact 128.750"). A clearance adds `cl-chip--clearance` (✓ and weight 600). No colour: green no longer means clearance.

**Speaker** `cl-chip cl-chip--atc` reads "ATC"; the pilot is the callsign chip; an unknown speaker is "?" in `color-text-tertiary`; an unknown speaker who named an aircraft shows the callsign chip alone, without "?" or arrow. An arrow (→) goes from speaker to addressee. The callsign chip is drawn 20 px high and clicked on 24.

**Data.** `speaker_type`, `callsign`, `plain[].text` (and `text_fr`) of `/api/v1/transcriptions`.

**Place** `cl-chip cl-chip--place` (Q55, 16/10): an official name from the AIP that the words probably were, "ELCOB 7G" for "elcope seven golf", "OPALE" for "Opal". Mono, dashed `color-border-control`, no fill: a proposal read from the sound, never a fact. It sits in the exchange's chip row after the instruction chips; the words it reads are underlined dotted in the transcript (`is-place`), which is never rewritten. Its help says what it is: "Official AIP name, proposed from the sound of the words: ELCOB 7G, a departure procedure. The transcript is left as heard." A point with coordinates is a `<button>`: it shows the point on the map for ten seconds; a procedure, a route, is not clickable. Shown only when it is very likely right: a procedure in the form name, digit, letter above the sound score's threshold, a point only after "direct", "proceed", "via", "hold" or "report" above the same threshold; the threshold is set on the lab's cases so that what is shown is right at least 95 times in 100. Otherwise nothing.
