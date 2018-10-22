package spinnakerdatadog_test

import (
	"os"
	"path/filepath"
	"testing"

	spinnakerdatadog "github.com/DataDog/spinnaker-datadog-bridge/spinnakerdatadog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCompilingTemplateTitleError(t *testing.T) {
	tmpl := &spinnakerdatadog.EventTemplate{
		Title: "{{ .Details.Bad } doing something",
		Text:  "{{ .Content.ExecutionID }} is the execution id",
		Tags: []string{
			"pipelineConfigId:{{ .Content.Execution.PipelineConfigID }}",
			"execution_status:{{ .Content.Execution.Status }}",
		},
	}
	err := tmpl.Compile()
	require.Error(t, err)
}

func TestCompilingTemplateEventError(t *testing.T) {
	tmpl := &spinnakerdatadog.EventTemplate{
		Title: "{{ .Details.Bad }} doing something",
		Text:  "{{ .Content.ExecutionID } is the execution id",
		Tags: []string{
			"pipelineConfigId:{{ .Content.Execution.PipelineConfigID }}",
			"execution_status:{{ .Content.Execution.Status }}",
		},
	}
	err := tmpl.Compile()
	require.Error(t, err)
}

func TestCompilingTemplateTagsError(t *testing.T) {
	tmpl := &spinnakerdatadog.EventTemplate{
		Title: "{{ .Details.Bad }} doing something",
		Text:  "{{ .Content.ExecutionID }} is the execution id",
		Tags: []string{
			"pipelineConfigId:{{ .Content.Execution.PipelineConfigID }",
			"execution_status:{{ .Content.Execution.Status }}",
		},
	}
	err := tmpl.Compile()
	require.Error(t, err)
}
