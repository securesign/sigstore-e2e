package kubernetes

import (
	"context"
	"strings"

	consoleV1 "github.com/openshift/api/console/v1"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

func ConsoleCLIDownload(ctx context.Context, c controller.Reader, cli string, os string, arch string) (string, error) {
	cld := &consoleV1.ConsoleCLIDownload{}
	ok := controller.ObjectKey{
		Name: cli,
	}
	err := c.Get(ctx, ok, cld)
	if err != nil {
		return "", err
	}
	var target string
	for _, link := range cld.Spec.Links {
		// Match old cli-server format (clients/<os>/<binary>-<arch>.gz)
		// and new content gateway format (<binary>_<os>_<arch>.tar.gz)
		matchOS := strings.Contains(link.Href, "/"+os+"/") || strings.Contains(link.Href, "_"+os+"_")
		matchArch := strings.Contains(link.Href, arch)

		if matchOS && matchArch {
			target = link.Href
		}
	}
	return target, nil
}
