package benchmark

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/securesign/sigstore-e2e/test/testsupport"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/registry"
)

func BenchmarkKeyless(b *testing.B) {

	logrus.SetLevel(logrus.InfoLevel)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := testsupport.CheckMandatoryAPIConfigValues(api.OidcRealm)
	if err != nil {
		b.Skip("Skip this test - " + err.Error())
	}

	registryContainer, err := registry.Run(context.Background(), api.GetValueFor(api.RegistryImage),
		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.Name = "registry"
			return nil
		}))

	if err != nil {
		b.Fatal("failed to start container", err)
	}

	// Clean up the container
	defer func() {
		if err := registryContainer.Terminate(ctx); err != nil {
			b.Fatal("failed to terminate container", err)
		}
	}()

	host, err := registryContainer.Host(ctx)
	if err != nil {
		b.Fatal("failed to get container host", err)
	}

	port, err := registryContainer.MappedPort(ctx, "5000")
	if err != nil {
		b.Fatal("failed to get container port", err)
	}

	registryURL := fmt.Sprintf("%s:%s", host, port)
	containerIP, _ := registryContainer.ContainerIP(ctx)

	// pool of images
	images := sync.Pool{
		New: func() any {
			ctx := context.Background()
			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			img := createRandomImage(timeoutCtx, registryURL)
			return strings.ReplaceAll(img, registryURL, containerIP+":5000")
		},
	}

	var poolSize int
	switch {
	case b.N <= runtime.GOMAXPROCS(0):
		poolSize = b.N
	case b.N > runtime.GOMAXPROCS(0) && runtime.GOMAXPROCS(0) < runtime.NumCPU():
		poolSize = runtime.GOMAXPROCS(0)
	case b.N > runtime.GOMAXPROCS(0) && runtime.GOMAXPROCS(0) > runtime.NumCPU():
		poolSize = runtime.NumCPU()
	default:
		poolSize = 1
	}

	pool, err := StartContainerPool(poolSize, func() (testcontainers.Container, error) {
		containerRequest := testcontainers.ContainerRequest{
			Image:      api.GetValueFor(api.CosignImage), // cosign image
			Entrypoint: []string{"sleep", "infinity"},    // Keep container running

			Env: map[string]string{
				"COSIGN_MIRROR":         api.GetValueFor(api.TufURL),
				"COSIGN_ROOT":           api.GetValueFor(api.TufURL) + "/root.json",
				"COSIGN_REKOR_URL":      api.GetValueFor(api.RekorURL),
				"COSIGN_FULCIO_URL":     api.GetValueFor(api.FulcioURL),
				"COSIGN_OIDC_ISSUER":    api.GetValueFor(api.OidcIssuerURL),
				"COSIGN_OIDC_CLIENT_ID": api.GetValueFor(api.OidcRealm),
			},
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: containerRequest,
			Started:          true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to start container: %w", err)
		}

		// wait to container be ready, sometime there has been problem with connection refused DNS
		time.Sleep(10 * time.Second)

		exitCode, reader, err := container.Exec(ctx, []string{"cosign", "initialize"})
		if err != nil {
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}

		rh := discard

		if logrus.IsLevelEnabled(logrus.DebugLevel) || exitCode != 0 {
			rh = toLogrus
		}

		// consume reader to prevent OOM
		go rh(reader)

		if exitCode != 0 {
			return nil, fmt.Errorf("failed to cosign initialize: %w, exit code: %d", err, exitCode)
		}

		return container, nil
	})
	if err != nil {
		b.Fatalf("failed to start container pool: %v", err)
	}
	defer pool.TerminatePool(ctx) // Ensure containers are terminated after the test

	tm := NewTokenManager(ctx)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err = keylessProcess(ctx, &images, pool, tm)
			if err != nil {
				b.Error(err)
			}
		}
	})
	b.StopTimer()
}

func keylessProcess(ctx context.Context, images *sync.Pool, containers *ContainerPool, tm *TokenManager) error {
	image, _ := images.Get().(string)
	defer images.Put(image) //nolint:staticcheck

	logrus.Info("sign: ", image)
	token := tm.GetToken()
	err := containers.ExecuteCommandInContainer(ctx, "cosign", "sign", "--allow-http-registry=true", "--upload=false", "-y", "--identity-token="+token, image)
	if err != nil {
		return err
	}
	return nil
}

func createRandomImage(ctx context.Context, registry string) string {
	image, err := random.Image(1024, 2)
	if err != nil {
		panic(err.Error())
	}

	imageName := uuid.New().String()

	ref, err := name.ParseReference(fmt.Sprintf("%s/%s", registry, imageName))
	if err != nil {
		panic(err.Error())
	}

	pusher, err := remote.NewPusher()
	if err != nil {
		panic(err.Error())
	}

	err = pusher.Push(ctx, ref, image)
	if err != nil {
		panic(err.Error())
	}
	hash, err := image.Digest()
	if err != nil {
		panic(err.Error())
	}

	return fmt.Sprintf("%s/%s@%s", registry, imageName, hash.String())
}
