package calfeed

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// agentLog records the User-Agent of every request a test server receives.
type agentLog struct {
	mu   sync.Mutex
	seen []string
}

func (a *agentLog) add(r *http.Request) {
	a.mu.Lock()
	a.seen = append(a.seen, r.Header.Get("User-Agent"))
	a.mu.Unlock()
}

func (a *agentLog) take() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := a.seen
	a.seen = nil
	return out
}

// Exchange on-premises (#622): 400 unless the User-Agent is a desktop browser's.
func TestFetchRetriesWithBrowserAgentAndRemembersIt(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()
	google, _ := os.ReadFile("testdata/google.ics")
	var log agentLog
	exchange := serve(t, func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Mozilla/5.0 (") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write(google)
	})
	w := &Worker{Store: s, Fetcher: localFetcher()}
	sub, err := s.Create(ctx, "u1", exchange.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res, err := w.Refresh(ctx, sub); err != nil || !res.Replaced || !res.BrowserAgent {
		t.Fatalf("first poll = %+v %v", res, err)
	}
	if got := log.take(); len(got) != 2 || got[0] != "OpenTaskKit/test" || got[1] != BrowserUserAgent {
		t.Fatalf("first poll agents = %q", got)
	}
	sub, _ = s.Get(ctx, "u1", sub.ID)
	if !sub.BrowserAgent || sub.LastError != "" || sub.EventCount == 0 {
		t.Fatalf("stored = %+v", sub)
	}
	if _, err := w.Refresh(ctx, sub); err != nil {
		t.Fatal(err)
	}
	if got := log.take(); len(got) != 1 || got[0] != BrowserUserAgent {
		t.Fatalf("second poll agents = %q", got)
	}
	// A new address starts over with the OpenTaskKit User-Agent.
	u := exchange.URL + "/other.ics"
	if sub, err = s.Update(ctx, "u1", sub.ID, Patch{URL: &u}); err != nil || sub.BrowserAgent {
		t.Fatalf("URL change = %+v %v", sub, err)
	}
}

func TestFetchBrowserRetryOnlyOnceAndOnlyForRefusals(t *testing.T) {
	f := localFetcher()
	ctx := context.Background()
	var log agentLog
	cases := []struct {
		code   int
		agents []string
	}{
		{400, []string{"OpenTaskKit/test", BrowserUserAgent}},
		{403, []string{"OpenTaskKit/test", BrowserUserAgent}},
		{406, []string{"OpenTaskKit/test", BrowserUserAgent}},
		{404, []string{"OpenTaskKit/test"}},
		{401, []string{"OpenTaskKit/test"}},
		{500, []string{"OpenTaskKit/test"}},
	}
	for _, c := range cases {
		srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
			log.add(r)
			w.WriteHeader(c.code)
		})
		if _, err := f.Fetch(ctx, &Subscription{URL: srv.URL}); err == nil || err.Error() != "http_"+strconv.Itoa(c.code) {
			t.Fatalf("%d = %v", c.code, err)
		}
		if got := log.take(); strings.Join(got, "|") != strings.Join(c.agents, "|") {
			t.Fatalf("%d agents = %q", c.code, got)
		}
	}
	// A subscription already on the browser agent is not retried either.
	refusing := serve(t, func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.WriteHeader(http.StatusBadRequest)
	})
	if _, err := f.Fetch(ctx, &Subscription{URL: refusing.URL, BrowserAgent: true}); err == nil || err.Error() != "http_400" {
		t.Fatalf("browser 400 = %v", err)
	}
	if got := log.take(); len(got) != 1 || got[0] != BrowserUserAgent {
		t.Fatalf("browser 400 agents = %q", got)
	}
}
