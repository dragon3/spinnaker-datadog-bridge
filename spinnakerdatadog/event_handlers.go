package spinnakerdatadog

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/bobbytables/spinnaker-datadog-bridge/spinnaker"
	"github.com/bobbytables/spinnaker-datadog-bridge/spinnaker/types"
	"github.com/pkg/errors"
	datadog "gopkg.in/zorkian/go-datadog-api.v2"
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

	event := &datadog.Event{}
	event.SetTitle(titleBuf.String())
	event.SetText(textBuf.String())
	event.SetAggregation(incoming.Content.ExecutionID)
	eventTypeDetails := strings.Split(incoming.Details.Type, ":")
	eventType := eventTypeDetails[1]
	eventStatus := eventTypeDetails[2]

	event.Tags = []string{
		"origin:spinnaker",
		fmt.Sprintf("app:%s", incoming.Details.Application),
		fmt.Sprintf("event_status:%s", eventStatus),
		fmt.Sprintf("event_type:%s", eventType),
		incoming.Details.Type,
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
