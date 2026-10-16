package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/yegors/co-atc/internal/aip"
	"github.com/yegors/co-atc/pkg/logger"
)

// The list of official names around the station (internal/aip, Q55), made
// from the settings page: its state, and a button that makes it again.

// AIPRadiusNM is how far around the station the reporting points are kept:
// the 120 NM the lab measured its list with (15/10).
const AIPRadiusNM = 120

// aipState is what the settings page shows.
type aipState struct {
	Present    bool     `json:"present"`
	AIRAC      string   `json:"airac,omitempty"`
	InForce    string   `json:"in_force"`
	NextAIRAC  string   `json:"next_airac,omitempty"`
	Made       string   `json:"made,omitempty"`
	Source     string   `json:"source,omitempty"`
	SourceURL  string   `json:"source_url,omitempty"`
	Points     int      `json:"points"`
	Callsigns  int      `json:"callsigns"`
	Procedures int      `json:"procedures"`
	RadiusNM   float64  `json:"radius_nm"`
	Airports   []string `json:"airports"`
	Skipped    []string `json:"skipped,omitempty"`
	Refreshing bool     `json:"refreshing"`
	LastError  string   `json:"last_error,omitempty"`
	LastTried  string   `json:"last_tried,omitempty"`
	AutoUpdate bool     `json:"auto_update"`

	// What the French eAIP covers of the airports followed: "france" (all),
	// "partial" or "none", and those outside it.
	Coverage string   `json:"coverage"`
	Outside  []string `json:"outside,omitempty"`

	// The procedures heard, named (Q55, usage 3): the points they are named
	// after, the coding tables read, the setting in force.
	ProcedurePoints int     `json:"procedure_points"`
	ChartsRead      int     `json:"charts_read"`
	ChartsNote      string  `json:"charts_note,omitempty"`
	NameProcedures  bool    `json:"name_procedures"`
	Naming          string  `json:"naming"`
	NamingFloor     float64 `json:"naming_floor"`
}

// AttachAIP gives the handler the list's maker; onMade is told of each new
// list (the matcher reads it).
func (h *Handler) AttachAIP(m *aip.Maker, onMade func(*aip.Names)) {
	h.aip = m
	h.aipMade = onMade
}

// aipRequest is the list as this station wants it: around the receiver, with
// the AD 2 pages of the airports it follows.
func (h *Handler) aipRequest() aip.Request {
	req := aip.Request{RadiusNM: AIPRadiusNM}
	if h.config != nil {
		req.Lat, req.Lon = h.config.Station.Latitude, h.config.Station.Longitude
	}
	if h.runtime != nil {
		req.Airports = h.runtime.Settings().ReferenceAirports()
	}
	return req
}

func (h *Handler) aipState() aipState {
	st := aipState{InForce: aip.InForce(time.Now()).Format("2006-01-02"), RadiusNM: AIPRadiusNM}
	st.Airports = h.aipRequest().Airports
	covered := 0
	for _, a := range st.Airports {
		if aip.Covered(a) {
			covered++
		} else {
			st.Outside = append(st.Outside, a)
		}
	}
	switch {
	case covered == 0:
		st.Coverage = "none"
	case covered < len(st.Airports):
		st.Coverage = "partial"
	default:
		st.Coverage = "france"
	}
	st.Naming = "measured"
	if h.runtime != nil {
		st.AutoUpdate = h.runtime.AIPAutoUpdate()
		st.NamingFloor, st.NameProcedures = h.runtime.PlaceNaming()
		if s := h.runtime.Settings().AIPNames; s != nil && s.Naming != "" {
			st.Naming = s.Naming
		}
	}
	if h.aip == nil {
		return st
	}
	if n, err := aip.Load(h.aip.OutPath); err == nil {
		st.Present = true
		st.AIRAC, st.NextAIRAC, st.Made = n.AIRAC, n.NextAIRAC, n.Made
		st.Source, st.SourceURL, st.Skipped = n.Source, n.SourceURL, n.Skipped
		st.Points, st.Callsigns, st.Procedures = len(n.Points), len(n.Callsigns), len(n.Procedures)
		st.RadiusNM = n.RadiusNM
		st.ProcedurePoints, st.ChartsRead, st.ChartsNote = len(n.ProcedurePoints), n.ChartsRead, n.ChartsNote
	}
	ms := h.aip.State()
	st.Refreshing, st.LastError = ms.Refreshing, ms.LastError
	if !ms.LastTried.IsZero() {
		st.LastTried = ms.LastTried.UTC().Format(time.RFC3339)
	}
	return st
}

// GetAIPNames says how the list stands.
func (h *Handler) GetAIPNames(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, h.aipState())
}

// PostAIPNamesRefresh makes the list again, in the background: five pages of
// the SIA, a few seconds. Behind the session group: it reaches an outside site.
func (h *Handler) PostAIPNamesRefresh(w http.ResponseWriter, r *http.Request) {
	if h.aip == nil {
		http.Error(w, "the list of official names is not available on this server", http.StatusServiceUnavailable)
		return
	}
	if h.aip.State().Refreshing {
		http.Error(w, aip.ErrBusy.Error(), http.StatusConflict)
		return
	}
	by := "anonymous"
	if sess, ok := h.auth.SessionFrom(r); ok {
		by = sess.User
	}
	h.logger.Info("Official names: update asked from the panel", logger.String("by", by))
	go h.makeAIP("panel")
	// The state says "refreshing" once the maker has started: wait for it, so
	// that the page's next read does not show the old state.
	for i := 0; i < 20 && !h.aip.State().Refreshing; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	WriteJSON(w, http.StatusAccepted, h.aipState())
}

func (h *Handler) makeAIP(why string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	started := time.Now()
	n, err := h.aip.Make(ctx, h.aipRequest())
	if err != nil {
		if !errors.Is(err, aip.ErrBusy) {
			h.logger.Warn("Official names: update failed", logger.String("why", why), logger.Error(err))
		}
		return
	}
	h.logger.Info("Official names: list made",
		logger.String("why", why),
		logger.String("airac", n.AIRAC),
		logger.Int("points", len(n.Points)),
		logger.Int("callsigns", len(n.Callsigns)),
		logger.Int("procedures", len(n.Procedures)),
		logger.String("took", time.Since(started).Round(time.Millisecond).String()))
	if h.aipMade != nil {
		h.aipMade(n)
	}
}

// KeepAIPNamesUpToDate makes the list again when a new cycle comes into force,
// while the panel's switch is on; checked every hour, and not more than once in
// six hours after a failure.
func (h *Handler) KeepAIPNamesUpToDate(ctx context.Context) {
	if h.aip == nil {
		return
	}
	check := func() {
		if h.runtime == nil || !h.runtime.AIPAutoUpdate() {
			return
		}
		st := h.aipState()
		if st.Present && st.AIRAC >= st.InForce || st.Coverage == "none" {
			return
		}
		if t := h.aip.State().LastTried; !t.IsZero() && time.Since(t) < 6*time.Hour {
			return
		}
		h.makeAIP("new AIRAC cycle")
	}
	go func() {
		check()
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				check()
			}
		}
	}()
}
