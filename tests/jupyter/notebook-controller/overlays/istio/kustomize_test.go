package istio

import (
	"github.com/kubeflow/manifests/tests"
	"testing"
)

func TestKustomize(t *testing.T) {
	testCase := &tests.KustomizeTestCase{
		Package:  "../../../../../jupyter/notebook-controller/overlays/istio",
		Expected: "test_data/expected",
	}

	tests.RunTestCase(t, testCase)
}
