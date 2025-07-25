// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package healthz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cilium/cilium/api/v1/models"
	"github.com/cilium/cilium/pkg/time"
)

type KubeProxyHealthzTestSuite struct{}

// Injected fake service.
type fakeLastUpdatedAter struct {
	injectedLastUpdatedTs time.Time
}

func (s *fakeLastUpdatedAter) GetLastUpdatedAt() time.Time {
	return s.injectedLastUpdatedTs
}

// Injected fake status collector
type FakeStatusCollector struct {
	injectedStatusResponse models.StatusResponse
}

func (d *FakeStatusCollector) GetStatus(brief bool, requireK8sConnectivity bool) models.StatusResponse {
	return d.injectedStatusResponse
}

type healthzPayload struct {
	LastUpdated string
	CurrentTime string
}

func TestKubeProxyHealth(t *testing.T) {
	s := KubeProxyHealthzTestSuite{}
	s.healthTestHelper(t, models.StatusStateOk, http.StatusOK, true)
	s.healthTestHelper(t, models.StatusStateWarning,
		http.StatusServiceUnavailable, false)
	s.healthTestHelper(t, models.StatusStateFailure,
		http.StatusServiceUnavailable, false)
}

func (s *KubeProxyHealthzTestSuite) healthTestHelper(t *testing.T, ciliumStatus string,
	expectedHttpStatus int, testcasepositive bool,
) {
	var lastUpdateTs, currentTs, expectedTs time.Time
	lastUpdateTs = time.Unix(100, 0) // Fake 100 seconds after Unix.
	currentTs = time.Unix(200, 0)    // Fake 200 seconds after Unix.
	prevTime := time.Now
	time.Now = func() time.Time {
		return currentTs
	}
	t.Cleanup(func() {
		time.Now = prevTime
	})

	expectedTs = lastUpdateTs
	if testcasepositive {
		expectedTs = currentTs
	}
	// Create handler with injected behavior.
	h := kubeproxyHealthzHandler{
		statusCollector: &FakeStatusCollector{injectedStatusResponse: models.StatusResponse{
			Cilium: &models.Status{State: ciliumStatus},
		}},
		lastUpdateAter: &fakeLastUpdatedAter{
			injectedLastUpdatedTs: lastUpdateTs,
		},
	}

	// Create a new request.
	req, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()

	// Serve.
	h.ServeHTTP(w, req)

	// Main return code meets expectations.
	require.Equalf(t, expectedHttpStatus, w.Code, "expected status code %v, got %v", expectedHttpStatus, w.Code)

	// Timestamps meet expectations.
	var payload healthzPayload
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	layout := "2006-01-02 15:04:05 -0700 MST"
	lastUpdateTs, err = time.Parse(layout, payload.LastUpdated)
	require.NoError(t, err)

	_, err = time.Parse(layout, payload.CurrentTime)
	require.NoError(t, err)
	require.True(t, lastUpdateTs.Equal(expectedTs))
}
