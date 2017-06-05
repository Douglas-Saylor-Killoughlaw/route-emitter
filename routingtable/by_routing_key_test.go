package routingtable_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/schema/endpoint"
	"code.cloudfoundry.org/routing-info/cfroutes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ByRoutingKey", func() {
	Describe("RoutesByRoutingKeyFromSchedulingInfos", func() {
		It("should build a map of routes", func() {
			abcRoutes := cfroutes.CFRoutes{
				{Hostnames: []string{"foo.com", "bar.com"}, Port: 8080, RouteServiceUrl: "https://something.creative", IsolationSegment: "test-isolation-segment"},
				{Hostnames: []string{"foo.example.com"}, Port: 9090},
			}
			defRoutes := cfroutes.CFRoutes{
				{Hostnames: []string{"baz.com"}, Port: 8080},
			}

			routes := routingtable.RoutesByRoutingKeyFromSchedulingInfos([]*models.DesiredLRPSchedulingInfo{
				{DesiredLRPKey: models.NewDesiredLRPKey("abc", "tests", "abc-guid"), Routes: abcRoutes.RoutingInfo()},
				{DesiredLRPKey: models.NewDesiredLRPKey("def", "tests", "def-guid"), Routes: defRoutes.RoutingInfo()},
			})

			Expect(routes).To(HaveLen(3))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].Hostname).To(Equal("foo.com"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].LogGuid).To(Equal("abc-guid"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].RouteServiceUrl).To(Equal("https://something.creative"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].IsolationSegment).To(Equal("test-isolation-segment"))

			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][1].Hostname).To(Equal("bar.com"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][1].LogGuid).To(Equal("abc-guid"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][1].RouteServiceUrl).To(Equal("https://something.creative"))

			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 9090}][0].Hostname).To(Equal("foo.example.com"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 9090}][0].LogGuid).To(Equal("abc-guid"))

			Expect(routes[endpoint.RoutingKey{ProcessGUID: "def", ContainerPort: 8080}][0].Hostname).To(Equal("baz.com"))
			Expect(routes[endpoint.RoutingKey{ProcessGUID: "def", ContainerPort: 8080}][0].LogGuid).To(Equal("def-guid"))
		})

		Context("when multiple hosts have the same key, but one hostname is bound to a route service and the other is not", func() {
			It("should build a map of routes", func() {
				abcRoutes := cfroutes.CFRoutes{
					{Hostnames: []string{"foo.com"}, Port: 8080, RouteServiceUrl: "https://something.creative"},
					{Hostnames: []string{"bar.com"}, Port: 8080},
				}
				defRoutes := cfroutes.CFRoutes{
					{Hostnames: []string{"baz.com"}, Port: 8080},
				}

				routes := routingtable.RoutesByRoutingKeyFromSchedulingInfos([]*models.DesiredLRPSchedulingInfo{
					{DesiredLRPKey: models.NewDesiredLRPKey("abc", "tests", "abc-guid"), Routes: abcRoutes.RoutingInfo()},
					{DesiredLRPKey: models.NewDesiredLRPKey("def", "tests", "def-guid"), Routes: defRoutes.RoutingInfo()},
				})

				Expect(routes).To(HaveLen(2))
				Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].Hostname).To(Equal("foo.com"))
				Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].LogGuid).To(Equal("abc-guid"))
				Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][0].RouteServiceUrl).To(Equal("https://something.creative"))

				Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][1].Hostname).To(Equal("bar.com"))
				Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][1].LogGuid).To(Equal("abc-guid"))
				Expect(routes[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 8080}][1].RouteServiceUrl).To(Equal(""))

				Expect(routes[endpoint.RoutingKey{ProcessGUID: "def", ContainerPort: 8080}][0].Hostname).To(Equal("baz.com"))
				Expect(routes[endpoint.RoutingKey{ProcessGUID: "def", ContainerPort: 8080}][0].LogGuid).To(Equal("def-guid"))
			})
		})
		Context("when the routing info is nil", func() {
			It("should not be included in the results", func() {
				routes := routingtable.RoutesByRoutingKeyFromSchedulingInfos([]*models.DesiredLRPSchedulingInfo{
					{DesiredLRPKey: models.NewDesiredLRPKey("abc", "tests", "abc-guid"), Routes: nil},
				})
				Expect(routes).To(HaveLen(0))
			})
		})
	})

	Describe("EndpointsByRoutingKeyFromActuals", func() {
		Context("when some actuals don't have port mappings", func() {
			var endpoints routingtable.EndpointsByRoutingKey

			BeforeEach(func() {
				schedInfo1 := model_helpers.NewValidDesiredLRP("abc").DesiredLRPSchedulingInfo()
				schedInfo1.Instances = 2
				schedInfo2 := model_helpers.NewValidDesiredLRP("def").DesiredLRPSchedulingInfo()
				schedInfo2.Instances = 2

				endpoints = routingtable.EndpointsByRoutingKeyFromActuals([]*endpoint.ActualLRPRoutingInfo{
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey(schedInfo1.ProcessGuid, 0, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("1.1.1.1", "1.2.3.4", models.NewPortMapping(11, 44), models.NewPortMapping(66, 99)),
						},
					},
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey(schedInfo1.ProcessGuid, 1, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("2.2.2.2", "2.3.4.5", models.NewPortMapping(22, 44), models.NewPortMapping(88, 99)),
						},
					},
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey(schedInfo2.ProcessGuid, 0, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("3.3.3.3", "3.4.5.6", models.NewPortMapping(33, 55)),
						},
					},
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey(schedInfo2.ProcessGuid, 1, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("4.4.4.4", "4.5.6.7", nil),
						},
					},
				}, map[string]*models.DesiredLRPSchedulingInfo{
					schedInfo1.ProcessGuid: &schedInfo1,
					schedInfo2.ProcessGuid: &schedInfo2,
				},
				)
			})

			It("should build a map of endpoints, ignoring those without ports", func() {
				Expect(endpoints).To(HaveLen(3))

				Expect(endpoints[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 44}]).To(ConsistOf([]routingtable.Endpoint{
					routingtable.Endpoint{
						Host:            "1.1.1.1",
						ContainerIP:     "1.2.3.4",
						Index:           0,
						Domain:          "domain",
						Port:            11,
						ContainerPort:   44,
						ModificationTag: &models.ModificationTag{},
					},
					routingtable.Endpoint{
						Host:            "2.2.2.2",
						ContainerIP:     "2.3.4.5",
						Index:           1,
						Domain:          "domain",
						Port:            22,
						ContainerPort:   44,
						ModificationTag: &models.ModificationTag{},
					}}))

				Expect(endpoints[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 99}]).To(ConsistOf([]routingtable.Endpoint{
					routingtable.Endpoint{
						Host:            "1.1.1.1",
						ContainerIP:     "1.2.3.4",
						Index:           0,
						Domain:          "domain",
						Port:            66,
						ContainerPort:   99,
						ModificationTag: &models.ModificationTag{},
					},

					routingtable.Endpoint{
						Host:            "2.2.2.2",
						ContainerIP:     "2.3.4.5",
						Index:           1,
						Domain:          "domain",
						Port:            88,
						ContainerPort:   99,
						ModificationTag: &models.ModificationTag{},
					}}))

				Expect(endpoints[endpoint.RoutingKey{ProcessGUID: "def", ContainerPort: 55}]).To(ConsistOf([]routingtable.Endpoint{
					routingtable.Endpoint{
						Host:            "3.3.3.3",
						ContainerIP:     "3.4.5.6",
						Index:           0,
						Domain:          "domain",
						Port:            33,
						ContainerPort:   55,
						ModificationTag: &models.ModificationTag{},
					}}))
			})
		})

		Context("when not all running actuals are desired", func() {
			var endpoints routingtable.EndpointsByRoutingKey

			BeforeEach(func() {
				schedInfo1 := model_helpers.NewValidDesiredLRP("abc").DesiredLRPSchedulingInfo()
				schedInfo1.Instances = 1
				schedInfo2 := model_helpers.NewValidDesiredLRP("def").DesiredLRPSchedulingInfo()
				schedInfo2.Instances = 1

				endpoints = routingtable.EndpointsByRoutingKeyFromActuals([]*endpoint.ActualLRPRoutingInfo{
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey("abc", 0, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("1.1.1.1", "1.2.3.4", models.NewPortMapping(11, 44), models.NewPortMapping(66, 99)),
						},
					},
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey("abc", 1, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("2.2.2.2", "2.3.4.5", models.NewPortMapping(22, 55), models.NewPortMapping(88, 99)),
						},
					},
					{
						ActualLRP: &models.ActualLRP{
							ActualLRPKey:     models.NewActualLRPKey("def", 0, "domain"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo("3.3.3.3", "3.4.5.6", models.NewPortMapping(33, 55)),
						},
					},
				}, map[string]*models.DesiredLRPSchedulingInfo{
					"abc": &schedInfo1,
					"def": &schedInfo2,
				},
				)
			})

			It("should build a map of endpoints, excluding actuals that aren't desired", func() {
				Expect(endpoints).To(HaveLen(3))

				Expect(endpoints[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 44}]).To(ConsistOf([]routingtable.Endpoint{
					routingtable.Endpoint{
						Host:            "1.1.1.1",
						ContainerIP:     "1.2.3.4",
						Domain:          "domain",
						Port:            11,
						ContainerPort:   44,
						ModificationTag: &models.ModificationTag{},
					}}))
				Expect(endpoints[endpoint.RoutingKey{ProcessGUID: "abc", ContainerPort: 99}]).To(ConsistOf([]routingtable.Endpoint{
					routingtable.Endpoint{
						Host:            "1.1.1.1",
						ContainerIP:     "1.2.3.4",
						Domain:          "domain",
						Port:            66,
						ContainerPort:   99,
						ModificationTag: &models.ModificationTag{},
					}}))
				Expect(endpoints[endpoint.RoutingKey{ProcessGUID: "def", ContainerPort: 55}]).To(ConsistOf([]routingtable.Endpoint{
					routingtable.Endpoint{
						Host:            "3.3.3.3",
						ContainerIP:     "3.4.5.6",
						Domain:          "domain",
						Port:            33,
						ContainerPort:   55,
						ModificationTag: &models.ModificationTag{},
					}}))
			})
		})

	})

	Describe("EndpointsFromActual", func() {
		It("builds a map of container port to endpoint", func() {
			endpoints, err := routingtable.EndpointsFromActual(&endpoint.ActualLRPRoutingInfo{
				ActualLRP: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.1.1.1", "1.2.3.4", models.NewPortMapping(11, 44), models.NewPortMapping(66, 99)),
				},
				Evacuating: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(endpoints).To(ConsistOf([]routingtable.Endpoint{
				routingtable.Endpoint{
					Host:            "1.1.1.1",
					ContainerIP:     "1.2.3.4",
					Domain:          "domain",
					Port:            11,
					InstanceGuid:    "instance-guid",
					ContainerPort:   44,
					Evacuating:      true,
					Index:           0,
					ModificationTag: &models.ModificationTag{},
				},
				routingtable.Endpoint{
					Host:            "1.1.1.1",
					ContainerIP:     "1.2.3.4",
					Domain:          "domain",
					Port:            66,
					InstanceGuid:    "instance-guid",
					ContainerPort:   99,
					Evacuating:      true,
					Index:           0,
					ModificationTag: &models.ModificationTag{},
				},
			}))
		})
	})

	Describe("RoutingKeysFromActual", func() {
		It("creates a list of keys for an actual LRP", func() {
			keys := routingtable.RoutingKeysFromActual(&models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.1.1.1", "1.2.3.4", models.NewPortMapping(11, 44), models.NewPortMapping(66, 99)),
			})
			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(endpoint.RoutingKey{ProcessGUID: "process-guid", ContainerPort: 44}))
			Expect(keys).To(ContainElement(endpoint.RoutingKey{ProcessGUID: "process-guid", ContainerPort: 99}))
		})

		Context("when the actual lrp has no port mappings", func() {
			It("returns no keys", func() {
				keys := routingtable.RoutingKeysFromActual(&models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.1.1.1", "1.2.3.4", nil),
				})

				Expect(keys).To(HaveLen(0))
			})
		})
	})

	Describe("RoutingKeysFromDesired", func() {
		It("creates a list of keys for an actual LRP", func() {
			routes := cfroutes.CFRoutes{
				{Hostnames: []string{"foo.com", "bar.com"}, Port: 8080},
				{Hostnames: []string{"foo.example.com"}, Port: 9090},
			}

			schedulingInfo := &models.DesiredLRPSchedulingInfo{
				DesiredLRPKey: models.NewDesiredLRPKey("process-guid", "tests", "abc-guid"),
				Routes:        routes.RoutingInfo(),
			}

			keys := routingtable.RoutingKeysFromSchedulingInfo(schedulingInfo)

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(endpoint.RoutingKey{ProcessGUID: "process-guid", ContainerPort: 8080}))
			Expect(keys).To(ContainElement(endpoint.RoutingKey{ProcessGUID: "process-guid", ContainerPort: 9090}))
		})

		Context("when the desired LRP does not define any container ports", func() {
			It("still uses the routes property", func() {
				schedulingInfo := &models.DesiredLRPSchedulingInfo{
					DesiredLRPKey: models.NewDesiredLRPKey("process-guid", "tests", "abc-guid"),
					Routes:        cfroutes.CFRoutes{{Hostnames: []string{"foo.com", "bar.com"}, Port: 8080}}.RoutingInfo(),
				}

				keys := routingtable.RoutingKeysFromSchedulingInfo(schedulingInfo)
				Expect(keys).To(HaveLen(1))
				Expect(keys).To(ContainElement(endpoint.RoutingKey{ProcessGUID: "process-guid", ContainerPort: 8080}))
			})
		})
	})
})
