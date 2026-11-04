package openshift

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"

	consoleV1 "github.com/openshift/api/console/v1"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/pkg/strategy"
	"github.com/securesign/sigstore-e2e/pkg/strategy/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// interstitialHTML stands in for the ~155 KB marketing page that the content
// gateway actually serves when a file URL is followed with redirects enabled.
const interstitialHTML = `<!DOCTYPE html><html lang="en"><body>Your download will begin shortly.</body></html>`

const (
	// publishedVersion stands for a release the content gateway hosts.
	publishedVersion = "1.5.0"
	// unpublishedVersion stands for a version a cluster running an unreleased
	// operator build can advertise before the gateway hosts it. These tests
	// run against a stub gateway and never touch the network, so the value is
	// narrative only — it is deliberately far enough ahead that the "not
	// published" framing does not go stale the moment the next z-stream ships.
	unpublishedVersion = "1.6.8"
)

// newContentGatewayStub reproduces the production content gateway:
//
//	GET /content-gateway/file/RHTAS/<publishedVersion>/<archive>
//	  -> 302 Location: /products/...?tcDownloadURL=<cdn link>
//	GET /products/trusted-artifact-signer
//	  -> 200 text/html  (the interstitial page, NOT the archive)
//	GET /cdn/<archive>
//	  -> 200 application/octet-stream (the real archive)
//
// The stub is version-aware: it only serves the archive under
// publishedVersion, 404ing any other version. This models production, where
// the content gateway only ever hosts published GA releases, while a
// ConsoleCLIDownload generated from an unreleased operator build can
// advertise a version the gateway has never published.
//
// Downloading the file URL directly therefore yields HTML. Verified against
// production: every CLI and every platform behaves this way.
func newContentGatewayStub(t *testing.T, publishedVersion string, archive string, payload []byte) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	wantPath := "/content-gateway/file/RHTAS/" + publishedVersion + "/" + archive
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/content-gateway/file/"):
			if r.URL.Path != wantPath {
				http.NotFound(w, r)
				return
			}
			cdn := srv.URL + "/cdn/" + archive
			loc := "/products/trusted-artifact-signer?success=true&tcDownloadURL=" + url.QueryEscape(cdn)
			http.Redirect(w, r, loc, http.StatusFound)

		case r.URL.Path == "/products/trusted-artifact-signer":
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			_, _ = w.Write([]byte(interstitialHTML))

		case r.URL.Path == "/cdn/"+archive:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(payload)

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func gatewayCLIDownload(archive, srvURL, version string) consoleV1.ConsoleCLIDownload {
	return consoleV1.ConsoleCLIDownload{
		ObjectMeta: metav1.ObjectMeta{Name: "rhtas-testcli"},
		Spec: consoleV1.ConsoleCLIDownloadSpec{
			DisplayName: "Test CLI",
			Description: "test binary",
			Links: []consoleV1.CLIDownloadLink{
				{Text: "Download testcli", Href: srvURL + "/content-gateway/file/RHTAS/" + version + "/" + archive},
			},
		},
	}
}

// TestContentGatewayResolvesInterstitial guards the fix for the defect where
// the suite extracted the HTML interstitial instead of the archive, fell back
// to a hardcoded older release, and so validated the wrong binaries.
func TestContentGatewayResolvesInterstitial(t *testing.T) {
	binary := []byte("#!/bin/sh\necho testcli\n")
	suffix := runtime.GOOS + "_" + runtime.GOARCH
	archive := "testcli_" + suffix + ".tar.gz"
	payload := testutil.BuildTarGz(t, map[string][]byte{"testcli_" + suffix: binary})

	srv := newContentGatewayStub(t, publishedVersion, archive, payload)
	fakeClient := newFakeClient(t, gatewayCLIDownload(archive, srv.URL, publishedVersion)).Build()

	path, err := download(t.Context(), fakeClient, "testcli", publishedVersion)
	if err != nil {
		t.Fatalf("download through the interstitial failed: %v", err)
	}
	testutil.VerifyBinary(t, path, binary)
}

// TestContentGatewayUsesRequestedVersion is the core regression case for
// CLI_CGW_VERSION: a cluster running an unreleased operator build advertises a
// ConsoleCLIDownload version (unpublishedVersion) that the content gateway has
// not published. The requested version (publishedVersion) must be used
// verbatim instead, with no fallback.
func TestContentGatewayUsesRequestedVersion(t *testing.T) {
	binary := []byte("#!/bin/sh\necho testcli\n")
	suffix := runtime.GOOS + "_" + runtime.GOARCH
	archive := "testcli_" + suffix + ".tar.gz"
	payload := testutil.BuildTarGz(t, map[string][]byte{"testcli_" + suffix: binary})

	srv := newContentGatewayStub(t, publishedVersion, archive, payload)
	fakeClient := newFakeClient(t, gatewayCLIDownload(archive, srv.URL, unpublishedVersion)).Build()

	path, err := download(t.Context(), fakeClient, "testcli", publishedVersion)
	if err != nil {
		t.Fatalf("download of the requested published version failed: %v", err)
	}
	testutil.VerifyBinary(t, path, binary)
}

// TestContentGatewayRequestedVersionNotPublishedFails pins the behaviour that
// matters most: when the requested version has not been published, the suite
// must fail loudly rather than quietly serving up some other version's
// artifact, and must not return a binary path.
func TestContentGatewayRequestedVersionNotPublishedFails(t *testing.T) {
	suffix := runtime.GOOS + "_" + runtime.GOARCH
	archive := "testcli_" + suffix + ".tar.gz"

	// The stub publishes only publishedVersion, and holds a usable archive for
	// it, so a fallback would succeed. Requesting unpublishedVersion must
	// still fail rather than quietly serve that one.
	srv := newContentGatewayStub(t, publishedVersion, archive, testutil.BuildTarGz(t, map[string][]byte{
		"testcli_" + suffix: []byte("stale binary from an older release\n"),
	}))
	fakeClient := newFakeClient(t, gatewayCLIDownload(archive, srv.URL, unpublishedVersion)).Build()

	path, err := download(t.Context(), fakeClient, "testcli", unpublishedVersion)
	if err == nil {
		t.Fatalf("expected a hard failure for an unpublished version, got %s", path)
	}
	if path != "" {
		t.Fatalf("expected no binary path on failure, got %q", path)
	}
}

// TestOpenshiftDefaultsToLatestPublished confirms that when CLI_CGW_VERSION
// is unset, the openshift strategy factory resolves to the package default
// (the latest published RHTAS GA, maintained in pkg/api/values.go) instead of
// erroring. RC/unpublished-binary testing is covered separately by
// CLI_STRATEGY=local + CLI_LOCAL_DIR, not by this strategy.
func TestOpenshiftDefaultsToLatestPublished(t *testing.T) {
	t.Setenv(api.CLICGWVersion, "")

	version := api.GetValueFor(api.CLICGWVersion)
	if version == "" {
		t.Fatal("expected CLI_CGW_VERSION to resolve to a default, got empty")
	}

	s, ok := strategy.Get("openshift")
	if !ok {
		t.Fatal("openshift strategy not registered")
	}
	if s == nil {
		t.Fatal("expected a usable strategy function even when CLI_CGW_VERSION is unset")
	}
}
