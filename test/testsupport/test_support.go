package testsupport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/kubernetes"
	"slices"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	consoleCli "github.com/openshift/api/console/v1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	olmV1 "github.com/operator-framework/api/pkg/operators/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	v1beta12 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonTriggers "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

const (
	Mandatory bool = true
	Optional  bool = false
)

var (
	TestContext       context.Context
	TestTimeoutMedium = 5 * time.Minute
)

var installedStack []api.TestPrerequisite = make([]api.TestPrerequisite, 0)

func init() {
	TestContext = context.TODO()

	var err error

	// Initialization of kubernetes client
	if kubernetes.K8sClient, err = kubernetes.NewClient(); err != nil {
		panic(err)
	}

	_ = olmV1alpha1.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = olmV1.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = projectv1.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = routev1.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = tektonTriggers.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = configv1.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = v1beta12.AddToScheme(kubernetes.K8sClient.GetScheme())
	_ = consoleCli.AddToScheme(kubernetes.K8sClient.GetScheme())

	logrus.SetFormatter(&logrus.TextFormatter{
		SortingFunc: func(s []string) {
			l := len(s)
			if l < 1 {
				return
			}

			i := slices.Index(s, "m")
			if i < 0 {
				return
			}

			s[l-1], s[i] = s[i], s[l-1]
		},
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "t",
			logrus.FieldKeyLevel: "l",
			logrus.FieldKeyMsg:   "m",
		},
		DisableQuote: true,
	})
}

func InstallPrerequisites(prerequisite ...api.TestPrerequisite) error {
	for _, p := range prerequisite {
		err := p.Setup(TestContext)
		if err != nil {
			return err
		}
		installedStack = append(installedStack, p)
	}
	return nil
}

func DestroyPrerequisites() error {
	var errors []error
	for i := len(installedStack) - 1; i >= 0; i-- {
		err := installedStack[i].Destroy(TestContext)
		if err != nil {
			logrus.Warn(err)
			errors = append(errors, err)
		}
	}
	if len(errors) != 0 {
		return fmt.Errorf("can't destroy all prerequisites %s", errors)
	}
	return nil
}

func GetOIDCToken(ctx context.Context, issuerURL string, userName string, password string, realm string) (string, error) {
	urlString := issuerURL + "/protocol/openid-connect/token"

	client := &http.Client{}
	data := url.Values{}
	data.Set("username", userName)
	data.Set("password", password)
	data.Set("scope", "openid")
	data.Set("client_id", realm)
	data.Set("grant_type", "password")

	r, _ := http.NewRequestWithContext(ctx, http.MethodPost, urlString, strings.NewReader(data.Encode())) // URL-encoded payload
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := client.Do(r)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(resp.Body)

	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	jsonOut := make(map[string]interface{})
	err = json.Unmarshal(b, &jsonOut)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", jsonOut["access_token"]), nil
}

func CheckApiConfigValues(failOnMissing bool, keys ...string) error {
	mandatoryMissing := false
	errorMessage := "Missing configuration for"
	for _, key := range keys {
		value := api.GetValueFor(key)
		if value == "" && failOnMissing {
			mandatoryMissing = true
			errorMessage += " " + key
			logrus.Warn(key, "=", value)
		} else {
			logrus.Info(key, "=", value)
		}
	}
	if mandatoryMissing {
		return errors.New(errorMessage)
	} else {
		return nil
	}
}
