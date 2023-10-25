package testSupport

import (
	"context"
	"encoding/json"
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	olmV1 "github.com/operator-framework/api/pkg/operators/v1"
	olmV1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	tektonTriggers "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	"io"
	"net/http"
	"net/url"
	"sigstore-e2e-test/pkg/client"
	"sigstore-e2e-test/pkg/support"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	TestClient        client.Client
	TestContext       context.Context
	TestTimeoutMedium = 5 * time.Minute
)

var prerequistities []support.TestPrerequisite

func init() {
	var err error

	TestContext = context.TODO()
	if TestClient, err = client.NewClient(); err != nil {
		panic(err)
	}

	olmV1alpha1.AddToScheme(TestClient.GetScheme())
	olmV1.AddToScheme(TestClient.GetScheme())
	projectv1.AddToScheme(TestClient.GetScheme())
	routev1.AddToScheme(TestClient.GetScheme())
	tektonTriggers.AddToScheme(TestClient.GetScheme())
	configv1.AddToScheme(TestClient.GetScheme())

}

func InstallPrerequisites(prerequisite ...support.TestPrerequisite) error {
	prerequistities = prerequisite
	wg := new(sync.WaitGroup)
	wg.Add(len(prerequisite))
	var errors []error
	for _, i := range prerequisite {
		go func(p support.TestPrerequisite) {
			err := p.Install(TestClient)
			if err != nil {
				errors = append(errors, err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	if len(errors) != 0 {
		return fmt.Errorf("can't install all prerequisities %s", errors)
	}
	return nil
}

func DestroyPrerequisites() error {
	wg := new(sync.WaitGroup)
	wg.Add(len(prerequistities))
	var errors []error
	for _, i := range prerequistities {
		go func(prerequisite support.TestPrerequisite) {
			err := prerequisite.Destroy(TestClient)
			if err != nil {
				logrus.Warn(err)
				errors = append(errors, err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
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
