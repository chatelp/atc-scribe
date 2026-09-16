package transcription

import "github.com/yegors/co-atc/internal/transcription/phraseology"

// FleetProvider is the aircraft the receiver can currently see.
//
// The matcher needs live ADS-B, but the transcription package must not depend on
// the ADS-B service: co-atc builds the two independently and an import here would
// tie them together for no gain. A one-method interface keeps the dependency
// pointing the right way — the wiring lives in cmd/server, and a test can supply
// a fixed fleet.
type FleetProvider interface {
	// Fleet returns the aircraft seen recently enough to plausibly be talking.
	// An empty result is normal: it means the sky is empty, or ADS-B is down.
	Fleet() []phraseology.Aircraft
}
