package phraseology

import "testing"

var lfpg = Sector{Lat: 49.0097, Lon: 2.5479, RadiusNM: 60, MaxAltFt: 20000}

func TestTheSectorKeepsOnlyWhatItCanBeTalkingTo(t *testing.T) {
	fleet := []Aircraft{
		{Callsign: "AFR23SB", AltitudeFt: 7000, Lat: 49.10, Lon: 2.60, HasPosition: true}, // 6 NM from Roissy, on approach
		{Callsign: "BAW334", AltitudeFt: 37000, Lat: 49.00, Lon: 2.50, HasPosition: true}, // overhead, cruising
		{Callsign: "RYR8XY", AltitudeFt: 9000, Lat: 47.20, Lon: -1.55, HasPosition: true}, // Nantes: 200 NM away
		{Callsign: "EZY36VJ", AltitudeFt: 5000},                                           // no position: kept
		{Callsign: "DLH36E", Lat: 49.05, Lon: 2.40, HasPosition: true},                    // no altitude: kept
	}
	var kept []string
	for _, ac := range lfpg.Within(fleet) {
		kept = append(kept, ac.Callsign)
	}
	want := []string{"AFR23SB", "EZY36VJ", "DLH36E"}
	if len(kept) != len(want) {
		t.Fatalf("kept %v, want %v", kept, want)
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Errorf("kept %v, want %v", kept, want)
		}
	}
}
