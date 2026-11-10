package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestDiscoveryKeepsDeniedEdgeVisible(t *testing.T) {
	for _, cause := range []error{os.ErrPermission, syscall.EPERM, syscall.EACCES} {
		t.Run(cause.Error(), func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "Microsoft Edge")
			denied := &os.PathError{Op: "readdir", Path: base, Err: cause}
			profiles := chromiumProfilesRead(runtime.GOOS, BrowserEdge, "Microsoft Edge", base, func(string) ([]os.DirEntry, error) { return nil, denied })
			if len(profiles) != 1 || profiles[0].BrowserID != BrowserEdge || !errors.Is(profiles[0].ReadError, cause) {
				t.Fatalf("denied Edge disappeared: %+v", profiles)
			}
			if profiles[0].Key() != "" || profiles[0].HasHistory() || profiles[0].HasBookmarks() {
				t.Fatal("invented a readable profile for denied directory")
			}
			if p, found := DefaultProfile(profiles, AutoTarget); !found || p.BrowserID != BrowserEdge {
				t.Fatal("denied browser reported missing")
			}
			discoveryFile(t, filepath.Join(base, "Default", "Bookmarks"), `{"roots":{"bookmark_bar":{"children":[{"type":"url","name":"Recovered Edge","url":"https://edge.example/"}]}}}`)
			retried := chromiumProfiles(runtime.GOOS, BrowserEdge, "Microsoft Edge", base)
			if len(retried) != 1 || retried[0].ReadError != nil || len(readBookmarks(retried[0])) != 1 {
				t.Fatalf("retry did not recover: %+v", retried)
			}
		})
	}
}

func TestDiscoveryIgnoresMissingBrowserButKeepsSeparateFailures(t *testing.T) {
	read := func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
	if got := chromiumProfilesRead(runtime.GOOS, BrowserEdge, "Microsoft Edge", t.TempDir(), read); len(got) != 0 {
		t.Fatalf("missing browser invented: %+v", got)
	}
	denied := []Profile{
		{BrowserID: BrowserEdge, BaseDir: filepath.Join(t.TempDir(), "one"), ReadError: os.ErrPermission},
		{BrowserID: BrowserEdge, BaseDir: filepath.Join(t.TempDir(), "two"), ReadError: os.ErrPermission},
	}
	if got := uniqueProfiles(append(denied, denied[0])); len(got) != 2 {
		t.Fatalf("inaccessible directories deduplicated incorrectly: %+v", got)
	}
}

func TestDeniedEdgeDoesNotHideReadableChromeOrFallBackToIt(t *testing.T) {
	base := t.TempDir()
	discoveryFile(t, filepath.Join(base, "Default", "Bookmarks"), `{"roots":{"bookmark_bar":{"children":[{"type":"url","name":"Chrome page","url":"https://chrome.example/"}]}}}`)
	profiles := chromiumProfiles(runtime.GOOS, BrowserChrome, "Chrome", base)
	profiles = append(profiles, Profile{BrowserID: BrowserEdge, Browser: "Microsoft Edge", BaseDir: t.TempDir(), ReadError: os.ErrPermission})
	if got := Search(context.Background(), SearchProfiles(profiles, AutoTarget), "", Options{}); len(got) != 1 || got[0].BrowserID != BrowserChrome {
		t.Fatalf("readable results lost: %+v", got)
	}
	if got := Search(context.Background(), SearchProfiles(profiles, BrowserEdge), "", Options{}); len(got) != 0 {
		t.Fatalf("Edge silently searched Chrome: %+v", got)
	}
	if got := SearchProfiles(profiles, BrowserEdgeBeta); len(got) != 0 {
		t.Fatalf("missing selected browser silently searched others: %+v", got)
	}
}
