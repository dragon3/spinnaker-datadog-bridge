package spinnakerdatadog_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DataDog/spinnaker-datadog-bridge/spinnaker"
	"github.com/DataDog/spinnaker-datadog-bridge/spinnaker/types"
	spinnakerdatadog "github.com/DataDog/spinnaker-datadog-bridge/spinnakerdatadog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	datadog "gopkg.in/zorkian/go-datadog-api.v2"
)

func TestSpoutInitialization(t *testing.T) {
	wd, _ := os.Getwd()

	t.Run("Given a valid template file", func(t *testing.T) {
		spout, err := spinnakerdatadog.NewSpout(nil, filepath.Join(wd, "testdata", "template.yml"))
		require.NoError(t, err)
		assert.Equal(t, 1, spout.TotalTemplates())
	})

	t.Run("Given a missing template file", func(t *testing.T) {
		_, err := spinnakerdatadog.NewSpout(nil, filepath.Join(wd, "testdata", "nope.yml"))
		require.Error(t, err)
	})

	t.Run("Given a badly formatted template file", func(t *testing.T) {
		_, err := spinnakerdatadog.NewSpout(nil, filepath.Join(wd, "testdata", "bad-format.yml"))
		require.Error(t, err)
	})
}

func TestAttachingToDispatcherForEvents(t *testing.T) {
	wd, _ := os.Getwd()

	d := spinnaker.NewDispatcher()
	spout, err := spinnakerdatadog.NewSpout(nil, filepath.Join(wd, "testdata", "template.yml"))
	require.NoError(t, err)

	spout.AttachToDispatcher(d)
	assert.Len(t, d.Handlers(), len(spout.Handlers()))
}

func TestEventDispatcherSendsDataDogFormattedEvents(t *testing.T) {
	mux := http.NewServeMux()
	var event datadog.Event
	done := make(chan error, 1)
	mux.HandleFunc("/api/v1/events", func(_ http.ResponseWriter, req *http.Request) {
		done <- json.NewDecoder(req.Body).Decode(&event)
	})
	ts := httptest.NewServer(mux)
	os.Setenv("DATADOG_HOST", ts.URL)
	defer os.Unsetenv("DATADOG_HOST")

	spout, _ := spinnakerdatadog.NewSpout(datadog.NewClient("", ""), "")
	template := &spinnakerdatadog.EventTemplate{
		Title: "{{ .Details.Application }} doing something",
		Text:  "{{ .Content.ExecutionID }} is the execution id",
		Tags: []string{
			"pipelineConfigId:{{ .Content.Execution.PipelineConfigID }}",
			"execution_status:{{ .Content.Execution.Status }}",
		},
	}

	handler := spinnakerdatadog.NewDatadogEventHandler(spout, template)
	err := handler.Handle(&types.IncomingWebhook{
		Details: types.Details{
			Application: "someapp",
			Type:        "orca:stage:failed",
		},
		Content: types.Content{
			ExecutionID: "someid",
			Execution: types.Execution{
				Status:           "TERMINAL",
				PipelineConfigID: "c6f20df7-f9ab-45b5-b525-9a67ef2e95b5",
			},
		},
	})

	require.NoError(t, err)

	select {
	case err := <-done:
		require.NoError(t, err, "error handling webhook")

		assert.Equal(t, "someapp doing something", event.GetTitle())
		assert.Equal(t, "someid is the execution id", event.GetText())
		assert.Equal(t, "origin:spinnaker", event.Tags[0])
		assert.Equal(t, "app:someapp", event.Tags[1])
		assert.Equal(t, "status:failed", event.Tags[2])
		assert.Equal(t, "type:stage", event.Tags[3])
		assert.Equal(t, "orca:stage:failed", event.Tags[4])
		assert.Equal(t, "pipelineConfigId:c6f20df7-f9ab-45b5-b525-9a67ef2e95b5", event.Tags[5])
		assert.Equal(t, "execution_status:TERMINAL", event.Tags[6])
	case <-time.After(time.Millisecond * 100):
		t.Error("timed out waiting for webhook call")
	}
}
