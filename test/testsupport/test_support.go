package testsupport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/securesign/sigstore-e2e/pkg/api"
	"github.com/sirupsen/logrus"
)

const (
	Mandatory bool = true
	Optional  bool = false
)

var (
	TestContext       context.Context
	TestTimeoutMedium = 5 * time.Minute
	// Config keys that must be defined for any test.
	mandatoryAPIConfigKeys = []string{api.OidcIssuerURL, api.FulcioURL, api.RekorURL, api.TufURL, api.TsaURL}
)

var installedStack []api.TestPrerequisite = make([]api.TestPrerequisite, 0)

func init() {
	TestContext = context.TODO()

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

func GetOIDCToken(ctx context.Context) (string, error) {
	if token := api.GetValueFor(api.OidcToken); token != "" {
		logrus.Info("Using OIDC token from ENV var")
		return token, nil
	}
	urlString := api.GetValueFor(api.OidcIssuerURL) + "/protocol/openid-connect/token"

	client := &http.Client{}
	data := url.Values{}
	data.Set("username", api.GetValueFor(api.OidcUser))
	data.Set("password", api.GetValueFor(api.OidcPassword))
	data.Set("scope", "openid")
	data.Set("client_id", api.GetValueFor(api.OidcRealm))
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

func CheckAnyTestMandatoryAPIConfigValues() error {
	return checkAPIConfigValues(Mandatory, mandatoryAPIConfigKeys...)
}

func CheckMandatoryAPIConfigValues(keys ...string) error {
	mandatoryAPIConfigKeys = append(mandatoryAPIConfigKeys, keys...)
	return checkAPIConfigValues(Mandatory, mandatoryAPIConfigKeys...)
}

func CheckOptionalAPIConfigValues(keys ...string) error {
	return checkAPIConfigValues(Optional, keys...)
}

func checkAPIConfigValues(failOnMissing bool, keys ...string) error {
	mandatoryMissing := false
	errorMessage := "Missing configuration for"
	if failOnMissing {
		logrus.Info("Mandatory configuration:")
	} else {
		logrus.Info("Optional configuration:")
	}
	for _, key := range keys {
		value := api.GetValueFor(key)
		if value == "" && failOnMissing {
			mandatoryMissing = true
			errorMessage += " " + key
			logrus.Warn(key, "=", value)
			hintPresent, hint := getHint(key)
			if hintPresent {
				logrus.Warn("   Hint: " + hint)
			}
		} else {
			logrus.Info(key, "=", value)
		}
	}
	if mandatoryMissing {
		logrus.Warn("   Hint: Missing config values should be provided during cluster installation. " +
			"Export them as environment variables.")
		return errors.New(errorMessage)
	} else {
		return nil
	}
}

func getHint(apiKey string) (bool, string) {
	if apiKey == api.GithubToken {
		return true, fmt.Sprintf("Authorization token for github client. Export it as environment variable %s.", apiKey)
	}
	return false, ""
}
