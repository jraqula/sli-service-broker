package sli_broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	. "github.com/pivotal-cf/brokerapi"
	datadog "github.com/zorkian/go-datadog-api"
)

type SliBroker struct {
	instances map[string]instance
	logger    lager.Logger
}

func NewSliBroker(logger lager.Logger) *SliBroker {
	return &SliBroker{
		instances: make(map[string]instance),
		logger:    logger,
	}
}

type instance struct {
	client *datadog.Client
	table  map[string]unbinder
}

var metricName = "cf_sli_service_up"
var host = "sliaas"

func (instance instance) emit(value float64, tags []string) {
	timeNow := float64(time.Now().Unix())

	instance.client.PostMetrics([]datadog.Metric{
		{
			Metric: &metricName,
			Points: []datadog.DataPoint{
				{&timeNow, &value},
			},
			Host: &host,
			Tags: tags,
		},
	})
}

// make something which can do this
type unbinder func()

var _ ServiceBroker = &SliBroker{}

func (b *SliBroker) Services(ctx context.Context) ([]Service, error) {
	return []Service{
		{
			ID:          "slis",
			Name:        "slis",
			Description: "indicates service level",
			Bindable:    true,
			Plans: []ServicePlan{
				{
					ID:          "seconds2",
					Name:        "seconds2",
					Description: "polls every 2 seconds",
				},
			},
		},
	}, nil
}

// '{"url": "https://..."}'
type csiParams struct {
	APIKey string `json:"dog_api_key"`
	AppKey string `json:"dog_app_key"`
}

func (b *SliBroker) Provision(ctx context.Context, instanceID string, details ProvisionDetails, asyncAllowed bool) (ProvisionedServiceSpec, error) {
	params := csiParams{}
	json.Unmarshal(details.RawParameters, &params)

	b.instances[instanceID] = instance{
		table:  make(map[string]unbinder),
		client: datadog.NewClient(params.APIKey, params.AppKey),
	}
	return ProvisionedServiceSpec{}, nil
}

func (b *SliBroker) Deprovision(ctx context.Context, instanceID string, details DeprovisionDetails, asyncAllowed bool) (DeprovisionServiceSpec, error) {
	if _, ok := b.instances[instanceID]; ok {
		delete(b.instances, instanceID)
	}
	return DeprovisionServiceSpec{}, nil
}

// '{"url": "https://..."}'
type bindParams struct {
	URL    string  `json:"url"`
	Source *string `json:"source,omitempty"`
}

func (b *SliBroker) Bind(ctx context.Context, instanceID, bindingID string, details BindDetails) (Binding, error) {
	params := bindParams{}
	json.Unmarshal(details.RawParameters, &params)

	instance := b.instances[instanceID]

	stopIt := make(chan struct{})

	go func() {
		tags := []string{
			fmt.Sprintf("service_instance_id:%s", instanceID),
			fmt.Sprintf("binding_id:%s", bindingID),
		}
		if params.Source != nil {
			tags = append(tags, fmt.Sprintf("source:%s", *params.Source))
		}

		for {
			select {
			case <-stopIt:
				return
			case <-time.After(1 * time.Second):
				resp, err := http.Get(params.URL)
				if err != nil {
					instance.emit(0, tags)
					continue
				}
				if resp.StatusCode >= 400 {
					instance.emit(0, tags)
					continue
				}
				instance.emit(1, tags)
			}
		}
	}()

	instance.table[bindingID] = func() {
		stopIt <- struct{}{}
	}
	return Binding{}, nil
}

func (b *SliBroker) Unbind(ctx context.Context, instanceID, bindingID string, details UnbindDetails) error {
	var f unbinder
	var ok bool
	if f, ok = b.instances[instanceID].table[bindingID]; ok {
		f()
	}
	return nil
}

func (b *SliBroker) Update(ctx context.Context, instanceID string, details UpdateDetails, asyncAllowed bool) (UpdateServiceSpec, error) {
	return UpdateServiceSpec{}, nil
}

func (b *SliBroker) LastOperation(ctx context.Context, instanceID, operationData string) (LastOperation, error) {
	return LastOperation{}, nil
}
