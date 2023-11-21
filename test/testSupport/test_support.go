package testSupport

import (
	"context"
	"encoding/json"
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	consoleCli "github.com/openshift/api/console/v1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	olmV1 "github.com/operator-framework/api/pkg/operators/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	v1beta12 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonTriggers "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	"io"
	"net/http"
	"net/url"
	"sigstore-e2e-test/pkg/api"
	"sigstore-e2e-test/pkg/kubernetes"
	"strconv"
	"strings"
	"time"
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

	olmV1alpha1.AddToScheme(kubernetes.K8sClient.GetScheme())
	olmV1.AddToScheme(kubernetes.K8sClient.GetScheme())
	projectv1.AddToScheme(kubernetes.K8sClient.GetScheme())
	routev1.AddToScheme(kubernetes.K8sClient.GetScheme())
	tektonTriggers.AddToScheme(kubernetes.K8sClient.GetScheme())
	configv1.AddToScheme(kubernetes.K8sClient.GetScheme())
	v1beta12.AddToScheme(kubernetes.K8sClient.GetScheme())
	consoleCli.AddToScheme(kubernetes.K8sClient.GetScheme())
}

func InstallPrerequisites(prerequisite ...api.TestPrerequisite) error {
	for _, p := range prerequisite {
		err := p.Setup()
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
		err := installedStack[i].Destroy()
		if err != nil {
			logrus.Warn(err)
			errors = append(errors, err)
		}
	}
	if len(errors) != 0 {
		return fmt.Errorf("can't destroy all prerequisities %s", errors)
	}
	return nil
}

func GetOIDCToken(issuerUrl string, userName string, password string, realm string) (string, error) {
	urlString := issuerUrl + "/protocol/openid-connect/token"

	client := &http.Client{}
	data := url.Values{}
	data.Set("username", userName)
	data.Set("password", password)
	data.Set("scope", "openid")
	data.Set("client_id", realm)
	data.Set("grant_type", "password")

	r, _ := http.NewRequest(http.MethodPost, urlString, strings.NewReader(data.Encode())) // URL-encoded payload
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
