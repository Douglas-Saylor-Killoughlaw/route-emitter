package endpoint_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routingtable/schema/endpoint"
	"code.cloudfoundry.org/routing-info/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRP Utils", func() {
	Describe("NewEndpointsFromActual", func() {
		Context("when actual is not evacuating", func() {
			It("builds a map of container port to endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}
				actualInfo := &endpoint.ActualLRPRoutingInfo{
					ActualLRP: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
							models.NewPortMapping(11, 44),
							models.NewPortMapping(66, 99),
						),
						State:           models.ActualLRPStateRunning,
						ModificationTag: tag,
					},
					Evacuating: false,
				}

				endpoints := endpoint.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(ConsistOf([]endpoint.Endpoint{
					endpoint.NewEndpoint("instance-guid", false, "1.1.1.1", 11, 44, &tag),
					endpoint.NewEndpoint("instance-guid", false, "1.1.1.1", 66, 99, &tag),
				}))
			})
		})

		Context("when actual is evacuating", func() {
			It("builds a map of container port to endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}

				actualInfo := &endpoint.ActualLRPRoutingInfo{
					ActualLRP: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
							models.NewPortMapping(11, 44),
							models.NewPortMapping(66, 99),
						),
						State:           models.ActualLRPStateRunning,
						ModificationTag: tag,
					},
					Evacuating: true,
				}

				endpoints := endpoint.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(ConsistOf([]endpoint.Endpoint{
					endpoint.NewEndpoint("instance-guid", true, "1.1.1.1", 11, 44, &tag),
					endpoint.NewEndpoint("instance-guid", true, "1.1.1.1", 66, 99, &tag),
				}))
			})
		})
	})

	Describe("NewRoutingKeysFromActual", func() {
		It("creates a list of keys for an actual LRP", func() {
			keys := endpoint.NewRoutingKeysFromActual(&endpoint.ActualLRPRoutingInfo{
				ActualLRP: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"1.1.1.1",
						models.NewPortMapping(11, 44),
						models.NewPortMapping(66, 99),
					),
					State: models.ActualLRPStateRunning,
				},
			})

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(endpoint.NewRoutingKey("process-guid", 44)))
			Expect(keys).To(ContainElement(endpoint.NewRoutingKey("process-guid", 99)))
		})

		Context("when the actual lrp has no port mappings", func() {
			It("returns no keys", func() {
				keys := endpoint.NewRoutingKeysFromActual(&endpoint.ActualLRPRoutingInfo{
					ActualLRP: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
						),
						State: models.ActualLRPStateRunning,
					},
				})

				Expect(keys).To(HaveLen(0))
			})
		})
	})

	Describe("NewRoutingKeysFromDesired", func() {
		It("creates a list of keys for an actual LRP", func() {
			routes := tcp_routes.TCPRoutes{
				{ExternalPort: 61000, ContainerPort: 8080},
				{ExternalPort: 61001, ContainerPort: 9090},
			}

			desired := (&models.DesiredLRP{
				Domain:      "tests",
				ProcessGuid: "process-guid",
				Ports:       []uint32{8080, 9090},
				Routes:      routes.RoutingInfo(),
				LogGuid:     "abc-guid",
			}).DesiredLRPSchedulingInfo()

			keys := endpoint.NewRoutingKeysFromDesired(&desired)

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(endpoint.NewRoutingKey("process-guid", 8080)))
			Expect(keys).To(ContainElement(endpoint.NewRoutingKey("process-guid", 9090)))
		})

		Context("when the desired LRP does not define any container ports", func() {
			It("returns no keys", func() {
				routes := tcp_routes.TCPRoutes{}

				desired := (&models.DesiredLRP{
					Domain:      "tests",
					ProcessGuid: "process-guid",
					Routes:      routes.RoutingInfo(),
					LogGuid:     "abc-guid",
				}).DesiredLRPSchedulingInfo()

				keys := endpoint.NewRoutingKeysFromDesired(&desired)
				Expect(keys).To(HaveLen(0))
			})
		})
	})
})
