package web_test

import (
	"os"
	"testing"

	"github.com/whywaita/myshoes/internal/testutils"
)

func TestMain(m *testing.M) {
	os.Exit(testutils.IntegrationTestRunner(m))
}
