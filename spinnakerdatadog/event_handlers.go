package spinnakerdatadog

import (
	"bytes"
	"fmt"
	"strings"

	dogstatsd "github.com/DataDog/datadog-go/statsd"
	"github.com/DataDog/spinnaker-datadog-bridge/spinnaker"
	"github.com/DataDog/spinnaker-datadog-bridge/spinnaker/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	datadogAPI "gopkg.in/zorkian/go-datadog-api.v2"
)

// DatadogEventHandler handles piping all of the registered events (via templates)
// to datadog when the dispatcher receives them. It handles compiling the original template
// and then sending it to the DataDog events API
type DatadogEventHandler struct {
	spout    *Spout
	template *EventTemplate
}

var _ spinnaker.Handler = (*DatadogEventHandler)(nil)

// NewDatadogEventHandler initializes a datadog event handler
func NewDatadogEventHandler(s *Spout, template *EventTemplate) *DatadogEventHandler {
	return &DatadogEventHandler{
		spout:    s,
		template: template,
	}
}

// Name implements spinnaker.Handler
func (deh *DatadogEventHandler) Name() string {
	return "DatadogEventHandler"
}

// Handle implements spinnaker.Handler. It sends datadog events for the given
// webhook event type. It compiles the given template from the webhook and sends it
func (deh *DatadogEventHandler) Handle(incoming *types.IncomingWebhook) error {
	if err := deh.template.Compile(); err != nil {
		return errors.Wrap(err, "could not compile template")
	}

	titleBuf, textBuf := new(bytes.Buffer), new(bytes.Buffer)
	if err := deh.template.compiledTitle.Execute(titleBuf, incoming); err != nil {
		return errors.Wrap(err, "could not compile title from webhook")
	}

	if err := deh.template.compiledText.Execute(textBuf, incoming); err != nil {
		return errors.Wrap(err, "could not compile text from webhook")
	}

	ddClient, err := dogstatsd.New("127.0.0.1:8125")
	if err != nil {
		return errors.Wrap(err, "could not open connection to dogstatsd")
	}

	ddClient.Namespace = "spinnaker."

	event := &datadogAPI.Event{}
	event.SetTitle(titleBuf.String())
	event.SetText(textBuf.String())
	event.SetAggregation(incoming.Content.ExecutionID)
	eventTypeDetails := strings.Split(incoming.Details.Type, ":")
	eventType := eventTypeDetails[1]
	eventStatus := eventTypeDetails[2]

	tags := []string{
		"origin:spinnaker",
		fmt.Sprintf("app:%s", incoming.Details.Application),
		fmt.Sprintf("status:%s", eventStatus),
		fmt.Sprintf("type:%s", eventType),
		incoming.Details.Type,
	}

	event.Tags = tags

	if eventType == "pipeline" && eventStatus != "starting" {
		metricTags := []string{
			fmt.Sprintf("execution_id:%s", incoming.Content.ExecutionID),
			fmt.Sprintf("triggered_by:%s", incoming.Content.Execution.Trigger.User),
			fmt.Sprintf("pipeline_name:%s", incoming.Content.Execution.Name),
		}
		metricTags = append(metricTags, tags...)

		duration := incoming.Content.Execution.EndTime.Sub(incoming.Content.Execution.StartTime.Time)
		err = ddClient.Timing("pipeline.duration", duration, metricTags, 1)

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("error submitting metric to datadog")
		} else {
			logrus.WithFields(logrus.Fields{
				"metric":   "pipeline.duration",
				"tags":     metricTags,
				"duration": duration.Seconds() * 1000,
			}).Info("submitted metric to datadog")
		}
	}

	if eventStatus == "failed" {
		event.SetAlertType("error")
	}

	for _, tag := range deh.template.compiledTags {
		tagBuf := new(bytes.Buffer)
		if err := tag.Execute(tagBuf, incoming); err != nil {
			return errors.Wrap(err, "could not compile tags from webhook")
		}
		event.Tags = append(event.Tags, tagBuf.String())
	}

	if _, err := deh.spout.client.PostEvent(event); err != nil {
		return errors.Wrap(err, "could not post to datadog API")
	}

	return nil
}
