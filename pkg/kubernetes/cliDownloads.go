package kubernetes

import (
	"context"
	consoleV1 "github.com/openshift/api/console/v1"
	"regexp"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

func ConsoleCLIDownload(ctx context.Context, cli string, os string) (string, error) {
	cld := &consoleV1.ConsoleCLIDownload{}
	ok := controller.ObjectKey{
		Name: cli,
	}
	err := K8sClient.Get(ctx, ok, cld)
	if err != nil {
		return "", err
	}
	var target string
	for _, link := range cld.Spec.Links {
		match, _ := regexp.MatchString("clients/"+os+"/", link.Href)
		if match {
			target = link.Href
		}
	}
	return target, nil
}
