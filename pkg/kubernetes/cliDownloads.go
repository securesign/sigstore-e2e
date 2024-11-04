package kubernetes

import (
	"context"
	"regexp"
	"strings"

	consoleV1 "github.com/openshift/api/console/v1"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

func ConsoleCLIDownload(ctx context.Context, cli string, os string, arch string) (string, error) {
	cld := &consoleV1.ConsoleCLIDownload{}
	ok := controller.ObjectKey{
		Name: cli,
	}
	err := GetClient().Get(ctx, ok, cld)
	if err != nil {
		return "", err
	}
	var target string
	for _, link := range cld.Spec.Links {
		matchOS, _ := regexp.MatchString("clients/"+os+"/", link.Href)
		matchArch := strings.Contains(link.Href, arch)

		if matchOS && matchArch {
			target = link.Href
		}
	}
	return target, nil
}
