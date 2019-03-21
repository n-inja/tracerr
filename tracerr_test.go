package tracerr_test

import (
	"testing"

	"github.com/n-inja/tracerr"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, tracerr.Analyzer, "a")
}
