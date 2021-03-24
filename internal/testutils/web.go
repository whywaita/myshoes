package testutils

// GetTestURL return url of httptest.Server
func GetTestURL() string {
	if testURL == "" {
		panic("testURL is not initialized yet")
	}

	return testURL
}
