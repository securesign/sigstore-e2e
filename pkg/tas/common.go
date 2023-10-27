package tas

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/sirupsen/logrus"
	"math/big"
	"sigstore-e2e-test/pkg/support"
	"time"
)

type SigstoreOcp struct {
	ctx       context.Context
	RepoDir   string
	gitUrl    string
	gitBranch string
}

func NewSigstoreOcp(ctx context.Context) *SigstoreOcp {
	return &SigstoreOcp{
		ctx:       ctx,
		gitUrl:    "https://github.com/securesign/sigstore-ocp",
		gitBranch: "release-1.0.beta",
	}
}

func (c *SigstoreOcp) Setup() error {
	var err error
	logrus.Info("Cloning sigstore-ocp project")
	c.RepoDir, _, err = support.GitClone(c.gitUrl, c.gitBranch)
	return err
}

func (c *SigstoreOcp) Destroy() error {
	//TODO delete temp files
	return nil
}

func (c *SigstoreOcp) IsReady() (bool, error) {
	if c.RepoDir == "" {
		return false, errors.New("sigstore-ocp project is not ready")
	} else {
		return true, nil
	}
}

func initCertificates(domain string, passwordProtected bool) ([]byte, []byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	// private
	privateKeyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	var block *pem.Block
	if passwordProtected {
		block, err = x509.EncryptPEMBlock(rand.Reader, "EC PRIVATE KEY", privateKeyBytes, []byte(CERT_PASSWORD), x509.PEMCipher3DES)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		block = &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: privateKeyBytes,
		}
	}
	privateKeyPem := pem.EncodeToMemory(block)

	// public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	publicKeyPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		},
	)

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * 10 * time.Hour)

	issuer := pkix.Name{
		CommonName:         domain,
		Country:            []string{"CR"},
		Organization:       []string{"RedHat"},
		Province:           []string{"Czech Republic"},
		Locality:           []string{"Brno"},
		OrganizationalUnit: []string{"QE"},
	}
	//Create certificate templet
	template := x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               issuer,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		Issuer:                issuer,
	}
	//Create certificate using templet
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err

	}
	//pem encoding of certificate
	root := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		},
	)
	return publicKeyPem, privateKeyPem, root, err
}
