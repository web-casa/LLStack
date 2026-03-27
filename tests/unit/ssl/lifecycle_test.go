package ssl_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/web-casa/llstack/internal/config"
	sslprovider "github.com/web-casa/llstack/internal/ssl"
)

func TestCertStatusOK(t *testing.T) {
	certPath := writeTempCert(t, time.Now().Add(90*24*time.Hour))
	lm := sslprovider.NewLifecycleManager(config.DefaultRuntimeConfig(), nil)
	statuses := lm.Status([]sslprovider.SiteInfo{
		{Name: "test.com", CertFile: certPath, Domain: "test.com"},
	})
	if len(statuses) != 1 || statuses[0].Status != "ok" {
		t.Fatalf("expected ok status, got %+v", statuses)
	}
	if statuses[0].DaysLeft < 80 {
		t.Fatalf("expected ~90 days left, got %d", statuses[0].DaysLeft)
	}
}

func TestCertStatusExpiring(t *testing.T) {
	certPath := writeTempCert(t, time.Now().Add(10*24*time.Hour))
	lm := sslprovider.NewLifecycleManager(config.DefaultRuntimeConfig(), nil)
	statuses := lm.Status([]sslprovider.SiteInfo{
		{Name: "expiring.com", CertFile: certPath},
	})
	if len(statuses) != 1 || statuses[0].Status != "expiring" {
		t.Fatalf("expected expiring status, got %+v", statuses)
	}
}

func TestCertStatusExpired(t *testing.T) {
	certPath := writeTempCert(t, time.Now().Add(-1*24*time.Hour))
	lm := sslprovider.NewLifecycleManager(config.DefaultRuntimeConfig(), nil)
	statuses := lm.Status([]sslprovider.SiteInfo{
		{Name: "expired.com", CertFile: certPath},
	})
	if len(statuses) != 1 || statuses[0].Status != "expired" {
		t.Fatalf("expected expired status, got %+v", statuses)
	}
}

func TestCertStatusMissing(t *testing.T) {
	lm := sslprovider.NewLifecycleManager(config.DefaultRuntimeConfig(), nil)
	statuses := lm.Status([]sslprovider.SiteInfo{
		{Name: "missing.com", CertFile: "/nonexistent/cert.pem"},
	})
	if len(statuses) != 1 || statuses[0].Status != "missing" {
		t.Fatalf("expected missing status, got %+v", statuses)
	}
}

func TestCertsExpiringSoon(t *testing.T) {
	statuses := []sslprovider.CertStatus{
		{Site: "ok.com", Status: "ok", DaysLeft: 90},
		{Site: "soon.com", Status: "expiring", DaysLeft: 10},
		{Site: "gone.com", Status: "expired", DaysLeft: -1},
	}
	expiring := sslprovider.CertsExpiringSoon(statuses, 14)
	if len(expiring) != 2 {
		t.Fatalf("expected 2 expiring, got %d", len(expiring))
	}
}

func writeTempCert(t *testing.T, notAfter time.Time) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     notAfter,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPath := filepath.Join(t.TempDir(), "cert.pem")
	f, _ := os.Create(certPath)
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	f.Close()
	return certPath
}
