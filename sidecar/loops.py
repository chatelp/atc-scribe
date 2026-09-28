"""Cut the loops out of a transcript, after the model.

Read once at temperature 0 (--fr-no-fallback), the French model keeps the loops that
mlx_whisper's temperature fallback used to break: on the benches of 27 and 28/09, a
4-gram repeated more than twice in 22.4 % and 12.0 % of French texts, against 9.8 % and
4.8 % with the fallback. The callsign matches did not show it; a measure of the text
itself did (whisper-lab, docs-fr/05-decisions.md, Q50).

Cutting where a run of words starts repeating back to back -- one occurrence kept, the
rest of the text dropped -- brings the loops to 4.2 % and 2.5 %, fewer than with the
fallback, at the cost of the fallback's extra decodes avoided. A single word must come
back four times in a row ("one one one" is a legitimate number), a group of two to
eight words three times.

Measured on the French reading only. On the English one the cut costs 1 to 4 true
matches for a smaller gain, and is not applied.
"""
import re
import unicodedata

_WORD = re.compile(r"[\w']+", re.UNICODE)


def _fold(w: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFD", w.lower())
                   if unicodedata.category(c) != "Mn")


def cut_loops(text: str) -> str:
    """The text up to the end of the first occurrence of its first loop, or the
    text unchanged when it has none."""
    spans = [(m.start(), m.end(), _fold(m.group())) for m in _WORD.finditer(text or "")]
    words = [s[2] for s in spans]
    for i in range(len(words)):
        for n in range(1, 9):
            reps = 4 if n == 1 else 3
            if i + n * reps > len(words):
                break
            if all(words[i + k * n:i + (k + 1) * n] == words[i:i + n] for k in range(1, reps)):
                cut = text[:spans[i + n - 1][1]].rstrip(" ,;-")
                return cut + ("." if text.rstrip().endswith(".") else "")
    return text
