"""python3 -m unittest sidecar/test_loops.py -- the standard library only."""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
from loops import cut_loops  # noqa: E402


class CutLoops(unittest.TestCase):
    def test_a_repeated_group_is_cut_after_its_first_occurrence(self):
        self.assertEqual(
            cut_loops("Air France 20, bonjour, niveau 100, niveau 100, niveau 100, niveau 100."),
            "Air France 20, bonjour, niveau 100.")

    def test_a_number_read_digit_by_digit_is_not_a_loop(self):
        text = "squawk one one one two"
        self.assertEqual(cut_loops(text), text)

    def test_a_single_word_needs_four_in_a_row(self):
        self.assertEqual(cut_loops("merci merci merci merci merci au revoir"), "merci")
        self.assertEqual(cut_loops("merci merci merci au revoir"), "merci merci merci au revoir")

    def test_accents_and_case_do_not_hide_a_loop(self):
        self.assertEqual(cut_loops("Réduisez 180, réduisez 180, REDUISEZ 180 nœuds"), "Réduisez 180")

    def test_a_text_without_loop_is_unchanged(self):
        text = "Air France 1234, descendez niveau 80, au revoir."
        self.assertEqual(cut_loops(text), text)
        self.assertEqual(cut_loops(""), "")


if __name__ == "__main__":
    unittest.main()
