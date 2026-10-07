package cli

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
)

// The fixture entrypoint exercises Execute/MCPCmd on macOS without changing
// system trust. Only the test binary injects its local CA into the HTTP client.
// The finite worker inherits SSL_CERT_FILE and uses the same entrypoint.
func TestMain(tests *testing.M) {
	if os.Getenv("BL_TEST_SKILLS_ENTRYPOINT") == "1" || os.Getenv(agentsetup.SkillsUpdateWorkerEnv) == "1" {
		certificate, err := os.ReadFile(os.Getenv("SSL_CERT_FILE"))
		if err != nil {
			panic(err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(certificate) {
			panic("invalid fixture certificate")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
		http.DefaultTransport = transport
		if err := Execute("0.1.121", "fixture", "fixture"); err != nil {
			_, _ = io.WriteString(os.Stderr, err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(tests.Run())
}
