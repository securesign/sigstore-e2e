package kubernetes

import (
	"context"
	"fmt"
	"strings"

	consoleV1 "github.com/openshift/api/console/v1"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

func ConsoleCLIDownload(ctx context.Context, c controller.Reader, cli string, os string, arch string) (string, error) {
	cld := &consoleV1.ConsoleCLIDownload{}

	// Try rhtas-prefixed name first (v1.5.0+), fall back to legacy name (v1.4.x).
	err := c.Get(ctx, controller.ObjectKey{Name: "rhtas-" + cli}, cld)
	if err != nil {
		err = c.Get(ctx, controller.ObjectKey{Name: cli}, cld)
	}
	if err != nil {
		return "", err
	}
	var target string
	for _, link := range cld.Spec.Links {
		// Match old cli-server format (clients/<os>/<binary>-<arch>.gz)
		// and new content gateway format (<binary>_<os>_<arch>.tar.gz)
		matchOS := strings.Contains(link.Href, "/"+os+"/") || strings.Contains(link.Href, "_"+os+"_")
		matchArch := strings.Contains(link.Href, "-"+arch+".") || strings.Contains(link.Href, "_"+arch+".")

		if matchOS && matchArch {
			target = link.Href
		}
	}
	if target == "" {
		return "", fmt.Errorf("no download link found for %s on %s/%s", cli, os, arch)
	}
	return target, nil
}
