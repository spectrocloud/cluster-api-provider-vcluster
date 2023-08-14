package vclustervalues

import (
	"strings"

	"github.com/ghodss/yaml"
	vclusterhelm "github.com/loft-sh/utils/pkg/helm"
	vclustervalues "github.com/loft-sh/utils/pkg/helm/values"
	"github.com/loft-sh/utils/pkg/log"
	"k8s.io/apimachinery/pkg/version"

	v1alpha1 "github.com/loft-sh/cluster-api-provider-vcluster/api/v1alpha1"
)

type Values interface {
	Merge(release *v1alpha1.VirtualClusterHelmRelease, logger log.Logger) (string, string, error)
}

func NewValuesMerger(kubernetesVersion *version.Info) Values {
	return &values{
		kubernetesVersion: kubernetesVersion,
	}
}

type values struct {
	kubernetesVersion *version.Info
}

func (v *values) Merge(release *v1alpha1.VirtualClusterHelmRelease, logger log.Logger) (string, string, error) {
	valuesObj := map[string]interface{}{}
	values := release.Values
	if values != "" {
		err := yaml.Unmarshal([]byte(values), &valuesObj)
		if err != nil {
			return "", "", err
		}
	}

	defaultValues, err := v.getVClusterDefaultValues(release, logger)
	if err != nil {
		return "", "", err
	}

	mergedValues := mergeMaps(defaultValues, valuesObj)

	out, err := yaml.Marshal(mergedValues)
	if err != nil {
		return "", "", err
	}
	finalValues := string(out)
	finalK8sVersion := getK8sImageVersionFromValues(finalValues)

	return finalK8sVersion, finalValues, nil
}

func (v *values) getVClusterDefaultValues(release *v1alpha1.VirtualClusterHelmRelease, logger log.Logger) (map[string]interface{}, error) {
	valuesStr, err := vclustervalues.GetDefaultReleaseValues(
		&vclusterhelm.ChartOptions{
			ChartName:    release.Chart.Name,
			ChartRepo:    release.Chart.Repo,
			ChartVersion: release.Chart.Version,
			KubernetesVersion: vclusterhelm.Version{
				Major: v.kubernetesVersion.Major,
				Minor: v.kubernetesVersion.Minor,
			},
		}, logger,
	)
	if err != nil {
		return nil, err
	}

	valuesObj := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(valuesStr), &valuesObj)
	if err != nil {
		return nil, err
	}

	return valuesObj, nil
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func getK8sImageVersionFromValues(values string) string {
	vclusterIdx := strings.Index(values, "vcluster:\n") // k0s, k3s
	if vclusterIdx == -1 {
		vclusterIdx = strings.Index(values, "api:\n") // k8s, eks
	}
	imageAnchor := "image:"
	imageIdx := strings.Index(values[vclusterIdx:], imageAnchor)

	start := vclusterIdx + imageIdx + len(imageAnchor)
	end := start + strings.Index(values[start:], "\n")
	image := strings.ReplaceAll(values[start:end], " ", "")
	valuesK8sVersion := strings.TrimPrefix(strings.Split(image, ":")[1], "v")

	return valuesK8sVersion
}
