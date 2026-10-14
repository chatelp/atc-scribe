// The sky's chrome (design system, MapControls and SegmentedControl;
// migration step 9): the base maps and layers of the menu behind the layers
// button in the map's corner. The settings, their saving in this browser and
// the methods that apply them are app.js's, unchanged; this lists them.
//
// One domain of the Alpine store 'atc' (docs-fr/32, palier 5.1); see
// store/station.js for how the domains are assembled.
(function () {
    window.AtcStore = window.AtcStore || [];
    window.AtcStore.push(() => ({
        // The layers menu (cl-menu--map), open or not.
        layersOpen: false,

        mapBaseStyles: [
            { id: 'ign-dark', label: 'IGN dark' },
            { id: 'ign', label: 'IGN' },
            { id: 'osm-dark', label: 'OSM dark' },
            { id: 'osm', label: 'OSM' },
            { id: 'none', label: 'None' },
        ],

        // One row per layer: its switch (a setting and the method applying
        // it) and, when it has one, its opacity.
        mapLayerRows: [
            { key: 'rings', label: 'Distance rings', show: 'showRings', toggle: 'toggleRings', op: 'ringsOpacity', setOp: 'setRingsOpacity',
                help: 'Circles every 10 NM around the station.' },
            { key: 'airspace', label: 'Airspace', show: 'showAirspaceBoundaries', toggle: 'toggleAirspaceBoundaries', op: 'airspaceOpacity', setOp: 'setAirspaceOpacity',
                help: 'The controlled airspace around the airports: CTR, TMA and their classes.' },
            { key: 'airports', label: 'Airports', show: 'showAirports', toggle: 'toggleAirports', op: 'airportsOpacity', setOp: 'setAirportsOpacity',
                help: 'Airports and airfields; a click opens the airport\'s sheet.' },
            { key: 'heliports', label: 'Heliports', show: 'showHeliports', toggle: 'toggleHeliports', op: 'heliportsOpacity', setOp: 'setHeliportsOpacity',
                help: 'Hospitals, police and private heliports.' },
            { key: 'navaids', label: 'Navaids', show: 'showNavaids', toggle: 'toggleNavaids', op: 'navaidsOpacity', setOp: 'setNavaidsOpacity',
                help: 'Radio beacons the aircraft navigate by: VOR, DME, NDB.' },
            { key: 'runways', label: 'Other runways', show: 'showAllRunways', toggle: 'toggleAllRunways', op: 'allRunwaysOpacity', setOp: 'setAllRunwaysOpacity',
                help: 'The runways of the airports not followed.' },
            { key: 'layout', label: 'Airport layout', show: 'showAirportLayout', toggle: 'toggleAirportLayout', op: null, setOp: null,
                help: 'Runways, taxiways, aprons and terminals of the airports near the station, from OpenStreetMap.' },
        ],

        setMapBase(id) {
            this.settings.mapStyle = id;
            this.setMapStyle();
        },

        // The app's toggles read the setting once it has changed (they were
        // bound to checkboxes).
        flipSetting(name, apply) {
            this.settings[name] = !this.settings[name];
            if (apply && typeof this[apply] === 'function') this[apply]();
        },

        setLayerOpacity(row, value) {
            this.settings[row.op] = Number(value);
            if (typeof this[row.setOp] === 'function') this[row.setOp]();
        },
    }));
})();
