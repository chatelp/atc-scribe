# ContextColumn

The context column, Right now tab: the default right-hand column of layout A.

**Composition.** `Tabs` (`cl-tabs--column`, fold button), then `cl-section` blocks: `SummarySentence`, `ArrivalSequence`, `EventList`, the counters (`cl-stats`: airborne, heard, on the ground). About 380 px wide on `color-surface-panel`, `color-border-default` on its left.

**Behaviour.** » folds it (layout B: Right now becomes a `cl-card` over the map); following a flight replaces its content with `AircraftStory`. Every block reads data the store already has (13/10).
