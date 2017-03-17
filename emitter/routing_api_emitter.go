package emitter

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-emitter/routingtable/schema/endpoint"
	"code.cloudfoundry.org/route-emitter/routingtable/schema/event"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/models"
	uaaclient "code.cloudfoundry.org/uaa-go-client"
)

//go:generate counterfeiter -o fakes/fake_routing_api_emitter.go . RoutingAPIEmitter
type RoutingAPIEmitter interface {
	Emit(routingEvents event.RoutingEvents) error
}

type routingAPIEmitter struct {
	logger           lager.Logger
	routingAPIClient routing_api.Client
	ttl              int
	uaaClient        uaaclient.Client
}

func NewRoutingAPIEmitter(logger lager.Logger, routingAPIClient routing_api.Client, uaaClient uaaclient.Client, routeTTL int) RoutingAPIEmitter {
	return &routingAPIEmitter{
		logger:           logger,
		routingAPIClient: routingAPIClient,
		ttl:              routeTTL,
		uaaClient:        uaaClient,
	}
}

func (t *routingAPIEmitter) Emit(tcpEvents event.RoutingEvents) error {
	t.logRoutingEvents(tcpEvents)
	defer t.logger.Debug("complete-emit")

	registrationMappingRequests, unregistrationMappingRequests := tcpEvents.ToMappingRequests(t.logger, t.ttl)
	t.emit(registrationMappingRequests, unregistrationMappingRequests)

	return nil
}

func (t *routingAPIEmitter) emit(registrationMappingRequests, unregistrationMappingRequests []models.TcpRouteMapping) error {
	emitted := true
	if len(registrationMappingRequests) > 0 {
		if err := t.routingAPIClient.UpsertTcpRouteMappings(registrationMappingRequests); err != nil {
			emitted = false
			t.logger.Error("unable-to-upsert", err)
			return err
		}
		t.logger.Debug("successfully-emitted-registration-events",
			lager.Data{"number-of-registration-events": len(registrationMappingRequests)})

	}

	if len(unregistrationMappingRequests) > 0 {
		if err := t.routingAPIClient.DeleteTcpRouteMappings(unregistrationMappingRequests); err != nil {
			emitted = false
			t.logger.Error("unable-to-delete", err)
			return err
		}
		t.logger.Debug("successfully-emitted-unregistration-events",
			lager.Data{"number-of-unregistration-events": len(unregistrationMappingRequests)})

	}

	if emitted {
		t.logger.Debug("successfully-emitted-events")
	}
	return nil
}

func (t *routingAPIEmitter) logRoutingEvents(routingEvents event.RoutingEvents) {
	for _, event := range routingEvents {
		endpoints := make([]endpoint.Endpoint, 0)
		for _, endpoint := range event.Entry.Endpoints {
			endpoints = append(endpoints, endpoint)
		}

		ports := make([]uint32, 0)
		for _, extEndpoint := range event.Entry.ExternalEndpoints {
			ports = append(ports, extEndpoint.Port)
		}
		t.logger.Info("mapped-routes", lager.Data{
			"external_ports": ports,
			"backends":       endpoints})
	}
}
