package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// startIn builds the service from the files in dir, as a start of the server
// would. Calling it twice on one dir is a restart.
func startIn(t *testing.T, dir string, loopback bool, proxies ...string) *Service {
	t.Helper()
	f, err := LoadUserFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("LoadUserFile: %v", err)
	}
	s, err := NewService(Config{UserFile: f, Loopback: loopback, TrustedProxies: proxies})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return s
}

// The bug: "this machine only" was answered on the first-run page, nothing was
// written, and the question came back after the reload -- with no way past it
// but creating an account.
func TestChoosingLocalIsRemembered(t *testing.T) {
	dir := t.TempDir()
	s := startIn(t, dir, true)
	if !s.NeedsSetup() {
		t.Fatal("precondition: a fresh server needs setting up")
	}

	if err := s.ChooseLocal(); err != nil {
		t.Fatalf("ChooseLocal: %v", err)
	}
	if s.NeedsSetup() {
		t.Error("the question was answered and is still being asked")
	}
	if s.Enabled() {
		t.Error("local-only must not require sign-in")
	}

	again := startIn(t, dir, true)
	if again.NeedsSetup() {
		t.Error("after a restart the question is asked again: the choice was not recorded")
	}
	if again.Enabled() {
		t.Error("after a restart sign-in is on, although local-only was chosen")
	}
	if got := again.Access().Mode; got != AccessLocal {
		t.Errorf("mode after restart: got %q, want %q", got, AccessLocal)
	}
}

// Local-only keeps the account, unused; choosing an account again brings it
// back with its password, and each state survives a restart.
func TestSwitchingBackAndForthKeepsTheAccount(t *testing.T) {
	dir := t.TempDir()
	s := startIn(t, dir, true)
	if err := s.AddUser("pierre", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}

	if err := s.ChooseLocal(); err != nil {
		t.Fatalf("ChooseLocal: %v", err)
	}
	if s.Enabled() || s.NeedsSetup() {
		t.Fatalf("local-only: enabled=%v needsSetup=%v, want false, false", s.Enabled(), s.NeedsSetup())
	}
	local := startIn(t, dir, true)
	if local.Enabled() {
		t.Error("local-only did not survive a restart")
	}
	if acc := local.Access().Accounts; len(acc) != 1 || acc[0] != "pierre" {
		t.Errorf("the account should be kept while local-only, got %v", acc)
	}

	if err := local.ChooseAccount(); err != nil {
		t.Fatalf("ChooseAccount: %v", err)
	}
	if !local.Enabled() {
		t.Error("choosing an account must turn sign-in back on")
	}
	if _, err := local.SignIn(loginRequest(), "pierre", "correcthorsebattery"); err != nil {
		t.Errorf("the kept account should sign in with its password: %v", err)
	}
	if !startIn(t, dir, true).Enabled() {
		t.Error("sign-in did not survive a restart")
	}
}

// Turning sign-in off on a server others can reach would open it to them.
func TestLocalIsRefusedWhenOthersCanReachTheServer(t *testing.T) {
	cases := []struct {
		name     string
		loopback bool
		proxies  []string
	}{
		{"bound to every interface", false, nil},
		{"loopback, behind a declared proxy", true, []string{"192.168.1.10"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			s := startIn(t, dir, c.loopback, c.proxies...)
			if err := s.AddUser("pierre", "correcthorsebattery"); err != nil {
				t.Fatal(err)
			}
			if s.LocalAllowed() {
				t.Error("local-only should not be offered here")
			}
			if err := s.ChooseLocal(); !errors.Is(err, ErrLocalNotAllowed) {
				t.Fatalf("ChooseLocal: got %v, want ErrLocalNotAllowed", err)
			}
			if !s.Enabled() {
				t.Error("the refusal turned sign-in off anyway")
			}
			if !startIn(t, dir, c.loopback, c.proxies...).Enabled() {
				t.Error("after a restart sign-in is off: the refused choice was written")
			}
		})
	}
}

// A choice made while only this machine could reach the server lapses once
// others can: the account is back in force, and with none the question is
// asked again -- which main turns into a refusal to start.
func TestALocalChoiceLapsesWhenTheServerIsExposed(t *testing.T) {
	dir := t.TempDir()
	s := startIn(t, dir, true)
	if err := s.AddUser("pierre", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}
	if err := s.ChooseLocal(); err != nil {
		t.Fatal(err)
	}
	exposed := startIn(t, dir, false)
	if !exposed.Enabled() {
		t.Error("exposed with local-only recorded, and sign-in is off: the server is open")
	}
	if !exposed.LocalRefused() {
		t.Error("the lapsed choice should be reported, so it can be logged")
	}

	empty := t.TempDir()
	if err := startIn(t, empty, true).ChooseLocal(); err != nil {
		t.Fatal(err)
	}
	if !startIn(t, empty, false).NeedsSetup() {
		t.Error("exposed, local-only recorded and no account: the server must count as unconfigured")
	}
}

func TestRequiringSignInNeedsSomeoneToSignIn(t *testing.T) {
	dir := t.TempDir()
	s := startIn(t, dir, true)
	if err := s.ChooseLocal(); err != nil {
		t.Fatal(err)
	}
	if err := s.ChooseAccount(); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("ChooseAccount with no account: got %v, want ErrNoAccount", err)
	}
	if s.Enabled() {
		t.Error("the refusal turned sign-in on anyway, with nobody to sign in as")
	}

	// Creating the first account from local-only is the switch.
	if err := s.AddUser("pierre", "correcthorsebattery"); err != nil {
		t.Fatalf("AddUser from local-only: %v", err)
	}
	if !s.Enabled() {
		t.Error("creating an account must turn sign-in on")
	}
	if !startIn(t, dir, true).Enabled() {
		t.Error("after a restart sign-in is off: local-only is still recorded beside the account")
	}
}

// A file written before the choice was recorded has accounts and no "access".
// The owner's own file is one: its account must keep turning sign-in on.
func TestAnAccountsFileWithoutAChoiceStillTurnsSignInOn(t *testing.T) {
	dir := t.TempDir()
	hash, err := HashPassword("correcthorsebattery")
	if err != nil {
		t.Fatal(err)
	}
	old := `{"users": [{"name": "pierre", "password_hash": "` + hash + `", "created_at": "2026-09-21T12:15:00Z"}]}`
	if err := os.WriteFile(filepath.Join(dir, DefaultUserFile), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s := startIn(t, dir, true)
	if !s.Enabled() || s.NeedsSetup() {
		t.Errorf("enabled=%v needsSetup=%v, want true, false", s.Enabled(), s.NeedsSetup())
	}
	if got := s.Access().Mode; got != AccessAccount {
		t.Errorf("mode: got %q, want %q", got, AccessAccount)
	}
}

// A choice that cannot be written must change nothing, or the server runs one
// way and restarts the other.
func TestAChoiceThatCannotBeWrittenChangesNothing(t *testing.T) {
	dir := t.TempDir()
	s := startIn(t, dir, true)
	if err := s.AddUser("pierre", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := s.ChooseLocal(); err == nil {
		t.Fatal("the choice could not be written and was reported as made")
	}
	if !s.Enabled() {
		t.Error("sign-in was turned off although the choice was not written")
	}
	if got := s.file.Access; got != AccessAccount {
		t.Errorf("the file's choice in memory changed to %q without being written", got)
	}
}

// Enabled is asked on every request while the panel can change the answer.
// Run with -race.
func TestSwitchingWhileRequestsArrive(t *testing.T) {
	s := startIn(t, t.TempDir(), true)
	if err := s.AddUser("pierre", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}
	h := s.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/aircraft", nil))
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		if err := s.ChooseLocal(); err != nil {
			t.Error(err)
		}
		if err := s.ChooseAccount(); err != nil {
			t.Error(err)
		}
	}
	close(stop)
	wg.Wait()
}
