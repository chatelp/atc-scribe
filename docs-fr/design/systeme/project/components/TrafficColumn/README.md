# TrafficColumn

The context column, Traffic tab: the flat list of aircraft and its filters; the map follows the filters.

**Composition.** `Tabs` (`cl-tabs--column`), then a `cl-section` of filters: `SearchField`, the altitude range (`Slider`, `cl-altrange`), Air / Ground / Heard and the five phase families (`FilterToggle`); then the list tabs (Active, Signal lost) with the sort, then `AircraftStrip`s.

**Behaviour.** The map shows "Showing 64 of 107 · SFC – FL 195" while a filter is on. Clicking a strip selects the aircraft; following it replaces the column with `AircraftStory`.
