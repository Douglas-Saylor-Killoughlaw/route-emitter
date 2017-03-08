package watcher_test

import (
	"errors"
	"os"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/schema/endpoint"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/route-emitter/watcher/fakes"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/tedsuo/ifrit"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

type EventHolder struct {
	event models.Event
}

var nilEventHolder = EventHolder{}

var _ = Describe("Watcher", func() {

	getDesiredLRP := func(processGuid, logGuid string,
		containerPort, externalPort uint32) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = []uint32{containerPort}
		desiredLRP.LogGuid = logGuid
		tcpRoutes := tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  externalPort,
				ContainerPort: containerPort,
			},
		}
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		return &desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint32, evacuating bool) *models.ActualLRPGroup {
		if evacuating {
			return &models.ActualLRPGroup{
				Instance: nil,
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State: models.ActualLRPStateRunning,
				},
			}
		} else {
			return &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State: models.ActualLRPStateRunning,
				},
				Evacuating: nil,
			}
		}
	}

	var (
		logger       lager.Logger
		eventSource  *eventfakes.FakeEventSource
		bbsClient    *fake_bbs.FakeClient
		routeHandler *fakes.FakeRouteHandler
		testWatcher  *watcher.Watcher
		clock        *fakeclock.FakeClock
		process      ifrit.Process
		cellID       string
		syncChannel  chan struct{}
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-watcher")
		eventSource = new(eventfakes.FakeEventSource)
		bbsClient = new(fake_bbs.FakeClient)
		routeHandler = new(fakes.FakeRouteHandler)

		clock = fakeclock.NewFakeClock(time.Now())
		bbsClient.SubscribeToEventsReturns(eventSource, nil)

		syncChannel = make(chan struct{})
		cellID = ""
		testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncChannel, logger)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("handle DesiredLRPCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = models.NewDesiredLRPCreatedEvent(desiredLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleEvent", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61001)
			event = models.NewDesiredLRPChangedEvent(beforeLRP, afterLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleEvent", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, changeEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPRemovedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = models.NewDesiredLRPRemovedEvent(desiredLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleDesiredDelete", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, deleteEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(deleteEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			event = models.NewActualLRPCreatedEvent(actualLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualCreate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			afterLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61001, 5222, false)
			event = models.NewActualLRPChangedEvent(beforeLRP, afterLRP)
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualUpdate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, changeEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("when an unrecognized event is received", func() {
		var (
			fakeRawEventSource *eventfakes.FakeRawEventSource
		)
		BeforeEach(func() {
			fakeRawEventSource = new(eventfakes.FakeRawEventSource)
			fakeEventSource := events.NewEventSource(fakeRawEventSource)

			fakeRawEventSource.NextReturns(
				sse.Event{
					ID:   "sup",
					Name: "unrecognized-event-type",
					Data: []byte("c3Nzcw=="),
				},
				nil,
			)

			bbsClient.SubscribeToEventsReturns(fakeEventSource, nil)
			testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncChannel, logger)
		})

		It("should not close the current connection", func() {
			Consistently(fakeRawEventSource.CloseCallCount).Should(Equal(0))
		})
	})

	Context("when eventSource returns error", func() {
		BeforeEach(func() {
			eventSource.NextReturns(nil, errors.New("bazinga..."))
		})

		It("closes the current event source", func() {
			Eventually(eventSource.CloseCallCount).Should(BeNumerically(">=", 1))
		})

		It("resubscribes to SSE from bbs", func() {
			Eventually(bbsClient.SubscribeToEventsCallCount, 5*time.Second, 300*time.Millisecond).Should(BeNumerically(">=", 2))
			Eventually(logger).Should(gbytes.Say("event-source-error"))
		})
	})

	Context("when subscribe to events fails", func() {
		var (
			bbsErrorChannel chan error
		)
		BeforeEach(func() {
			bbsErrorChannel = make(chan error)

			bbsClient.SubscribeToEventsStub = func(logger lager.Logger) (events.EventSource, error) {
				select {
				case err := <-bbsErrorChannel:
					if err != nil {
						return nil, err
					}
				}
				return eventSource, nil
			}

			testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncChannel, logger)
		})

		JustBeforeEach(func() {
			bbsErrorChannel <- errors.New("kaboom")
		})

		It("retries to subscribe", func() {
			close(bbsErrorChannel)
			Eventually(bbsClient.SubscribeToEventsCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(2))
			Eventually(logger).Should(gbytes.Say("kaboom"))
		})
	})

	Describe("Sync Events", func() {
		var (
			ready            chan struct{}
			errCh            chan error
			eventCh          chan EventHolder
			fakeMetricSender *fake_metrics_sender.FakeMetricSender
		)

		BeforeEach(func() {
			ready = make(chan struct{})
			errCh = make(chan error, 10)
			eventCh = make(chan EventHolder, 1)
			// make the variables local to avoid race detection
			nextErr := errCh
			nextEventValue := eventCh

			eventSource.CloseStub = func() error {
				nextErr <- errors.New("closed")
				return nil
			}

			eventSource.NextStub = func() (models.Event, error) {
				t := time.After(10 * time.Millisecond)
				select {
				case err := <-nextErr:
					return nil, err
				case x := <-nextEventValue:
					return x.event, nil
				case <-t:
					return nil, nil
				}
			}
			fakeMetricSender = fake_metrics_sender.NewFakeMetricSender()
			metrics.Initialize(fakeMetricSender, nil)
		})

		currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
		hostname1 := "foo.example.com"
		hostname2 := "bar.example.com"
		hostname3 := "baz.example.com"
		endpoint1 := routingtable.Endpoint{InstanceGuid: "ig-1", Host: "1.1.1.1", Index: 0, Port: 11, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
		endpoint2 := routingtable.Endpoint{InstanceGuid: "ig-2", Host: "2.2.2.2", Index: 0, Port: 22, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
		endpoint3 := routingtable.Endpoint{InstanceGuid: "ig-3", Host: "2.2.2.2", Index: 1, Port: 23, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}

		schedulingInfo1 := &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("pg-1", "tests", "lg1"),
			Routes: cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames:       []string{hostname1},
					Port:            8080,
					RouteServiceUrl: "https://rs.example.com",
				},
			}.RoutingInfo(),
			Instances: 1,
		}

		schedulingInfo2 := &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("pg-2", "tests", "lg2"),
			Routes: cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames: []string{hostname2},
					Port:      8080,
				},
			}.RoutingInfo(),
			Instances: 1,
		}

		schedulingInfo3 := &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("pg-3", "tests", "lg3"),
			Routes: cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames: []string{hostname3},
					Port:      8080,
				},
			}.RoutingInfo(),
			Instances: 1,
		}

		actualLRPGroup1 := &models.ActualLRPGroup{
			Instance: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGuid, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			},
		}

		actualLRPGroup2 := &models.ActualLRPGroup{
			Instance: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-2", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint2.InstanceGuid, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint2.Host, models.NewPortMapping(endpoint2.Port, endpoint2.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			},
		}

		actualLRPGroup3 := &models.ActualLRPGroup{
			Instance: &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGuid, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint3.Host, models.NewPortMapping(endpoint3.Port, endpoint3.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			},
		}

		sendEvent := func() {
			Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRPGroup1)}))
		}

		JustBeforeEach(func() {
			syncChannel <- struct{}{}
		})

		Describe("bbs events", func() {
			var count int32

			BeforeEach(func() {
				ready = make(chan struct{})
				count = 0

				bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					defer GinkgoRecover()

					atomic.AddInt32(&count, 1)
					ready <- struct{}{}
					Eventually(ready).Should(Receive())
					return nil, nil
				}
			})

			JustBeforeEach(func() {
				Eventually(ready).Should(Receive())
			})

			It("caches events", func() {
				sendEvent()
				Consistently(routeHandler.HandleEventCallCount).Should(Equal(0))
				ready <- struct{}{}
			})

			It("applies cached events after syncing is complete", func() {
				sendEvent()
				ready <- struct{}{}
				Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
				_, event := routeHandler.HandleEventArgsForCall(0)

				expectedEvent := models.NewActualLRPRemovedEvent(actualLRPGroup1)
				Expect(event).To(Equal(expectedEvent))
			})

			Context("when still syncing", func() {
				It("ignores a sync event", func() {
					var sentSync bool
					select {
					case syncChannel <- struct{}{}:
						sentSync = true
					default:
					}

					Expect(sentSync).To(BeFalse())
					ready <- struct{}{}
				})
			})
		})

		Context("when fetching actuals fails", func() {
			var returnError int32

			BeforeEach(func() {
				returnError = 1

				bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					if atomic.LoadInt32(&returnError) == 1 {
						return nil, errors.New("bam")
					}

					return []*models.ActualLRPGroup{}, nil
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				atomic.StoreInt32(&returnError, 0)
				syncChannel <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.ActualLRPGroupsCallCount()).To(Equal(2))
			})
		})

		Context("when fetching desireds fails", func() {
			var returnError int32

			BeforeEach(func() {
				returnError = 1

				bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
					if atomic.LoadInt32(&returnError) == 1 {
						return nil, errors.New("bam")
					}

					return []*models.DesiredLRPSchedulingInfo{}, nil
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				atomic.StoreInt32(&returnError, 0)
				syncChannel <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.DesiredLRPSchedulingInfosCallCount()).To(Equal(2))
			})
		})

		Context("when fetching domains fails", func() {
			var returnError int32

			BeforeEach(func() {
				returnError = 1

				bbsClient.DomainsStub = func(logger lager.Logger) ([]string, error) {
					if atomic.LoadInt32(&returnError) == 1 {
						return nil, errors.New("bam")
					}

					return []string{}, nil
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.DomainsCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				atomic.StoreInt32(&returnError, 0)
				syncChannel <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.DomainsCallCount()).To(Equal(2))
			})

			It("does not emit the sync duration metric", func() {
				Consistently(func() float64 {
					return fakeMetricSender.GetValue("RouteEmitterSyncDuration").Value
				}).Should(BeZero())
			})
		})

		Context("when desired lrps are retrieved", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
					clock.IncrementBySeconds(1)

					return []*models.ActualLRPGroup{
						actualLRPGroup1,
						actualLRPGroup2,
						actualLRPGroup3,
					}, nil
				}

				ready = make(chan struct{})

				bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
					defer GinkgoRecover()

					ready <- struct{}{}
					Eventually(ready).Should(Receive())

					return []*models.DesiredLRPSchedulingInfo{schedulingInfo1, schedulingInfo2}, nil
				}
			})

			It("calls RouteHandler Sync with correct arguments", func() {
				Eventually(ready).Should(Receive())
				ready <- struct{}{}
				expectedDesired := []*models.DesiredLRPSchedulingInfo{
					schedulingInfo1,
					schedulingInfo2,
				}
				expectedActuals := []*endpoint.ActualLRPRoutingInfo{
					endpoint.NewActualLRPRoutingInfo(actualLRPGroup1),
					endpoint.NewActualLRPRoutingInfo(actualLRPGroup2),
					endpoint.NewActualLRPRoutingInfo(actualLRPGroup3),
				}

				expectedDomains := models.DomainSet{}
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, desired, actuals, domains := routeHandler.SyncArgsForCall(0)

				Expect(domains).To(Equal(expectedDomains))
				Expect(desired).To(Equal(expectedDesired))
				Expect(actuals).To(Equal(expectedActuals))
			})

			It("should emit the sync duration, and allow event processing", func() {
				Eventually(ready).Should(Receive())
				ready <- struct{}{}
				Eventually(func() float64 {
					return fakeMetricSender.GetValue("RouteEmitterSyncDuration").Value
				}).Should(BeNumerically(">=", 100*time.Millisecond))

				By("completing, events are no longer cached")
				sendEvent()

				Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
			})

			It("gets all the desired lrps", func() {
				Eventually(ready).Should(Receive())
				ready <- struct{}{}
				Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(1))
				_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(0)
				Expect(filter.ProcessGuids).To(BeEmpty())
			})

			Context("when the cell id is set", func() {
				BeforeEach(func() {
					cellID = "another-cell-id"
					actualLRPGroup2.Instance.ActualLRPInstanceKey.CellId = cellID

					testWatcher = watcher.NewWatcher(cellID, bbsClient, clock, routeHandler, syncChannel, logger)
				})

				Context("when the cell has actual lrps running", func() {
					BeforeEach(func() {
						bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
							clock.IncrementBySeconds(1)

							return []*models.ActualLRPGroup{
								actualLRPGroup2,
							}, nil
						}
						bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
							defer GinkgoRecover()
							ready <- struct{}{}
							Eventually(ready).Should(Receive())
							return []*models.DesiredLRPSchedulingInfo{schedulingInfo2}, nil
						}
					})

					JustBeforeEach(func() {
						Eventually(ready).Should(Receive())
						ready <- struct{}{}
					})

					It("calls Sync method with correct desired lrps", func() {
						Eventually(routeHandler.SyncCallCount).Should(Equal(1))
						_, desired, _, _ := routeHandler.SyncArgsForCall(0)
						Expect(desired).To(ContainElement(schedulingInfo2))
						Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
					})

					It("registers endpoints for lrps on this cell", func() {
						Eventually(routeHandler.SyncCallCount).Should(Equal(1))
						_, _, actual, _ := routeHandler.SyncArgsForCall(0)
						routingInfo2 := endpoint.NewActualLRPRoutingInfo(actualLRPGroup2)
						Expect(actual).To(ContainElement(routingInfo2))
					})

					It("fetches actual lrps that match the cell id", func() {
						Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
						_, filter := bbsClient.ActualLRPGroupsArgsForCall(0)
						Expect(filter.CellID).To(Equal(cellID))
					})

					It("fetches desired lrp scheduling info that match the cell id", func() {
						Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
						_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(0)
						lrp, _ := actualLRPGroup2.Resolve()
						Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))
					})
				})

				Context("when desired lrp for the actual lrp is missing", func() {
					BeforeEach(func() {
						cellID = "cell-id"

						bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
							defer GinkgoRecover()
							ready <- struct{}{}
							Eventually(ready).Should(Receive())
							if len(f.ProcessGuids) == 1 && f.ProcessGuids[0] == "pg-3" {
								return []*models.DesiredLRPSchedulingInfo{schedulingInfo3}, nil
							}
							return []*models.DesiredLRPSchedulingInfo{schedulingInfo1}, nil
						}

						bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
							clock.IncrementBySeconds(1)
							return []*models.ActualLRPGroup{actualLRPGroup1}, nil
						}

						routeHandler.ShouldRefreshDesiredReturns(true)
					})

					Context("when a running actual lrp event is received", func() {
						JustBeforeEach(func() {
							Eventually(ready).Should(Receive())
							ready <- struct{}{}

							beforeActualLRPGroup3 := &models.ActualLRPGroup{
								Instance: &models.ActualLRP{
									ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
									ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGuid, "cell-id"),
									State:                models.ActualLRPStateClaimed,
								},
							}

							Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPChangedEvent(
								beforeActualLRPGroup3,
								actualLRPGroup3,
							)}))
						})

						It("fetches the desired lrp and passes it to the route handler", func() {
							Eventually(ready).Should(Receive())
							ready <- struct{}{}

							Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(2))

							_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(1)
							lrp, _ := actualLRPGroup3.Resolve()

							Expect(filter.ProcessGuids).To(HaveLen(1))
							Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))

							Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
							Eventually(routeHandler.RefreshDesiredCallCount).Should(Equal(1))
							_, desiredInfo := routeHandler.RefreshDesiredArgsForCall(0)
							Expect(desiredInfo).To(ContainElement(schedulingInfo3))

							Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
						})

						Context("when fetching desired scheduling info fails", func() {
							BeforeEach(func() {
								bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
									defer GinkgoRecover()
									ready <- struct{}{}
									Eventually(ready).Should(Receive())
									return nil, errors.New("blam!")
								}
							})

							It("does not refresh the desired state", func() {
								Eventually(ready).Should(Receive())
								ready <- struct{}{}

								Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
								Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(2))
								Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
							})
						})
					})

					Context("when actual lrp state is not running", func() {
						BeforeEach(func() {
							actualLRPGroup4 := &models.ActualLRPGroup{
								Instance: &models.ActualLRP{
									ActualLRPKey:         models.NewActualLRPKey("pg-4", 1, "domain"),
									ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGuid, "cell-id"),
									State:                models.ActualLRPStateClaimed,
								},
							}

							Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPCreatedEvent(
								actualLRPGroup4,
							)}))
						})

						JustBeforeEach(func() {
							Eventually(ready).Should(Receive())
							ready <- struct{}{}
						})

						It("should not refresh desired lrps", func() {
							Consistently(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(0))
							Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
						})
					})

					Context("when there are no running actual lrps on the cell", func() {
						BeforeEach(func() {
							bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
								clock.IncrementBySeconds(1)

								ready <- struct{}{}
								Eventually(ready).Should(Receive())

								return []*models.ActualLRPGroup{}, nil
							}
						})

						JustBeforeEach(func() {
							Eventually(ready).Should(Receive())
							ready <- struct{}{}
						})

						It("does not fetch any desired lrp scheduling info", func() {
							Consistently(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(0))
						})
					})
				})
			})
		})
	})
})
