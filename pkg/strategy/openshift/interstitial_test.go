package openshift

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"

	consoleV1 "github.com/openshift/api/console/v1"
	"github.com/securesign/sigstore-e2e/pkg/strategy/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// interstitialHTML stands in for the ~155 KB marketing page that the content
// gateway actually serves when a file URL is followed with redirects enabled.
const interstitialHTML = `<!DOCTYPE html><html lang="en"><body>Your download will begin shortly.</body></html>`

// newContentGatewayStub reproduces the production content gateway:
//
//	GET /content-gateway/file/RHTAS/<ver>/<archive>
//	  -> 302 Location: /products/...?tcDownloadURL=<cdn link>
//	GET /products/trusted-artifact-signer
//	  -> 200 text/html  (the interstitial page, NOT the archive)
//	GET /cdn/<archive>
//	  -> 200 application/octet-stream (the real archive)
//
// Downloading the file URL directly therefore yields HTML. Verified against
// production: every CLI and every platform behaves this way.
func newContentGatewayStub(t *testing.T, archive string, payload []byte) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/content-gateway/file/"):
			if !strings.HasSuffix(r.URL.Path, archive) {
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

	srv := newContentGatewayStub(t, archive, payload)
	fakeClient := newFakeClient(t, gatewayCLIDownload(archive, srv.URL, "1.5.0")).Build()

	path, err := download(t.Context(), fakeClient, "testcli")
	if err != nil {
		t.Fatalf("download through the interstitial failed: %v", err)
	}
	testutil.VerifyBinary(t, path, binary)
}

// TestContentGatewayMissingArtifactFails pins the behaviour that matters most:
// when the release under test does not publish the binary, the suite must fail
// loudly rather than quietly serving up some other release's artifact.
func TestContentGatewayMissingArtifactFails(t *testing.T) {
	suffix := runtime.GOOS + "_" + runtime.GOARCH
	archive := "testcli_" + suffix + ".tar.gz"

	// The stub only knows about 1.4.2; the CLI download advertises 1.5.0.
	srv := newContentGatewayStub(t, archive, testutil.BuildTarGz(t, map[string][]byte{
		"testcli_" + suffix: []byte("stale binary from an older release\n"),
	}))
	missing := "testcli_" + suffix + "_missing.tar.gz"
	fakeClient := newFakeClient(t, gatewayCLIDownload(missing, srv.URL, "1.5.0")).Build()

	if path, err := download(t.Context(), fakeClient, "testcli"); err == nil {
		t.Fatalf("expected a hard failure for an unpublished artifact, got %s", path)
	}
}
