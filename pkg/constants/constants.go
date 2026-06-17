package constants

import "os"

var (
	// DefaultVClusterVersion is the default version of the virtual cluster to use for NEW clusters.
	// Existing clusters keep the version pinned on their VCluster CR (e.g. 0.27.3), and the matching
	// bundled chart is retained under charts/ for backward compatibility.
	DefaultVClusterVersion = "0.34.1"

	// DefaultVClusterChartName is the default chart name of the virtual cluster to use
	DefaultVClusterChartName = "vcluster"

	GenericVClusterChartName = "vcluster-generic"

	// DefaultVClusterRepo is the default repo url of the virtual cluster to use
	DefaultVClusterRepo = "https://charts.loft.sh"
)

func init() {
	if os.Getenv("DEFAULT_VCLUSTER_CHART_VERSION") != "" {
		DefaultVClusterVersion = os.Getenv("DEFAULT_VCLUSTER_CHART_VERSION")
	}
	if os.Getenv("DEFAULT_VCLUSTER_CHART_NAME") != "" {
		DefaultVClusterChartName = os.Getenv("DEFAULT_VCLUSTER_CHART_NAME")
	}
	if os.Getenv("DEFAULT_VCLUSTER_CHART_REPO") != "" {
		DefaultVClusterRepo = os.Getenv("DEFAULT_VCLUSTER_CHART_REPO")
	}
}
