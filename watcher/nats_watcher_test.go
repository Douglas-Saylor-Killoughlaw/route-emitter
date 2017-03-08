package watcher_test

//import (
//	"encoding/json"
//	"errors"
//	"fmt"
//	"os"
//	"sync/atomic"
//	"time"
//
//	"code.cloudfoundry.org/bbs/events"
//	"code.cloudfoundry.org/bbs/events/eventfakes"
//	"code.cloudfoundry.org/bbs/fake_bbs"
//	"code.cloudfoundry.org/bbs/models"
//	"code.cloudfoundry.org/clock/fakeclock"
//	"code.cloudfoundry.org/lager"
//	"code.cloudfoundry.org/lager/lagertest"
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//	. "github.com/onsi/gomega/gbytes"
//	"github.com/tedsuo/ifrit"
//
//	"code.cloudfoundry.org/route-emitter/emitter/fakes"
//	"code.cloudfoundry.org/route-emitter/routingtable"
//	"code.cloudfoundry.org/route-emitter/routingtable/fake_routingtable"
//	"code.cloudfoundry.org/route-emitter/syncer"
//	"code.cloudfoundry.org/route-emitter/watcher"
//	"code.cloudfoundry.org/routing-info/cfroutes"
//	fake_metrics_sender "github.com/cloudfoundry/dropsonde/metric_sender/fake"
//	"github.com/cloudfoundry/dropsonde/metrics"
//)
//
//const logGuid = "some-log-guid"
//
//type EventHolder struct {
//	event models.Event
//}
//
//var nilEventHolder = EventHolder{}
//
//var _ = Describe("Watcher", func() {
//	const (
//		expectedDomain                  = "domain"
//		expectedProcessGuid             = "process-guid"
//		expectedInstanceGuid            = "instance-guid"
//		expectedIndex                   = 0
//		expectedHost                    = "1.1.1.1"
//		expectedExternalPort            = 11000
//		expectedAdditionalExternalPort  = 22000
//		expectedContainerPort           = 11
//		expectedAdditionalContainerPort = 22
//		expectedRouteServiceUrl         = "https://so.good.com"
//	)
//
//	var (
//		eventSource *eventfakes.FakeEventSource
//		bbsClient   *fake_bbs.FakeClient
//		fakeTable   *fake_routingtable.FakeRoutingTable
//		table       routingtable.RoutingTable
//		natsEmitter *fakes.FakeNATSEmitter
//		syncEvents  syncer.Events
//		cellID      string
//
//		clock          *fakeclock.FakeClock
//		watcherProcess *watcher.Watcher
//		process        ifrit.Process
//
//		expectedRoutes     []string
//		expectedRoutingKey routingtable.RoutingKey
//		expectedCFRoute    cfroutes.CFRoute
//
//		expectedAdditionalRoutes     []string
//		expectedAdditionalRoutingKey routingtable.RoutingKey
//		expectedAdditionalCFRoute    cfroutes.CFRoute
//
//		dummyMessagesToEmit routingtable.MessagesToEmit
//		fakeMetricSender    *fake_metrics_sender.FakeMetricSender
//
//		logger *lagertest.TestLogger
//
//		errCh   chan error
//		eventCh chan EventHolder
//	)
//
//	BeforeEach(func() {
//		eventSource = new(eventfakes.FakeEventSource)
//		bbsClient = new(fake_bbs.FakeClient)
//		bbsClient.SubscribeToEventsReturns(eventSource, nil)
//		bbsClient.DomainsReturns([]string{expectedDomain}, nil)
//		cellID = ""
//
//		fakeTable = &fake_routingtable.FakeRoutingTable{}
//		table = fakeTable
//		natsEmitter = &fakes.FakeNATSEmitter{}
//		syncEvents = syncer.Events{
//			Sync: make(chan struct{}),
//			Emit: make(chan struct{}),
//		}
//		logger = lagertest.NewTestLogger("test")
//
//		dummyEndpoint := routingtable.Endpoint{InstanceGuid: expectedInstanceGuid, Index: expectedIndex, Host: expectedHost, Port: expectedContainerPort}
//		dummyMessageFoo := routingtable.RegistryMessageFor(dummyEndpoint, routingtable.Route{Hostname: "foo.com", LogGuid: logGuid})
//		dummyMessageBar := routingtable.RegistryMessageFor(dummyEndpoint, routingtable.Route{Hostname: "bar.com", LogGuid: logGuid})
//		dummyMessagesToEmit = routingtable.MessagesToEmit{
//			RegistrationMessages: []routingtable.RegistryMessage{dummyMessageFoo, dummyMessageBar},
//		}
//
//		clock = fakeclock.NewFakeClock(time.Now())
//
//		expectedRoutes = []string{"route-1", "route-2"}
//		expectedCFRoute = cfroutes.CFRoute{Hostnames: expectedRoutes, Port: expectedContainerPort, RouteServiceUrl: expectedRouteServiceUrl}
//		expectedRoutingKey = routingtable.RoutingKey{
//			ProcessGuid:   expectedProcessGuid,
//			ContainerPort: expectedContainerPort,
//		}
//
//		expectedAdditionalRoutes = []string{"additional-1", "additional-2"}
//		expectedAdditionalCFRoute = cfroutes.CFRoute{Hostnames: expectedAdditionalRoutes, Port: expectedAdditionalContainerPort}
//		expectedAdditionalRoutingKey = routingtable.RoutingKey{
//			ProcessGuid:   expectedProcessGuid,
//			ContainerPort: expectedAdditionalContainerPort,
//		}
//		fakeMetricSender = fake_metrics_sender.NewFakeMetricSender()
//		metrics.Initialize(fakeMetricSender, nil)
//
//		errCh = make(chan error, 10)
//		eventCh = make(chan EventHolder, 1)
//
//		// make the variables local to avoid race detection
//		nextErr := errCh
//		nextEventValue := eventCh
//
//		eventSource.CloseStub = func() error {
//			nextErr <- errors.New("closed")
//			return nil
//		}
//
//		eventSource.NextStub = func() (models.Event, error) {
//			t := time.After(10 * time.Millisecond)
//			select {
//			case err := <-nextErr:
//				return nil, err
//			case x := <-nextEventValue:
//				return x.event, nil
//			case <-t:
//				return nil, nil
//			}
//		}
//	})
//
//	JustBeforeEach(func() {
//		watcherProcess = watcher.NewWatcher(
//			cellID,
//			bbsClient,
//			clock,
//			table,
//			natsEmitter,
//			syncEvents,
//			logger,
//		)
//
//		process = ifrit.Invoke(watcherProcess)
//	})
//
//	AfterEach(func() {
//		process.Signal(os.Interrupt)
//		Eventually(process.Wait()).Should(Receive())
//	})
//
//	Context("on startup", func() {
//		It("processes events after the first sync event", func() {
//			Consistently(bbsClient.SubscribeToEventsCallCount).Should(Equal(0))
//			syncEvents.Sync <- struct{}{}
//			Eventually(bbsClient.SubscribeToEventsCallCount).Should(BeNumerically(">", 0))
//		})
//	})
//
//	Describe("Desired LRP changes", func() {
//		behaveAsDesired := func() {
//			JustBeforeEach(func() {
//				syncEvents.Sync <- struct{}{}
//				Eventually(natsEmitter.EmitCallCount).ShouldNot(Equal(0))
//			})
//
//			Context("when a create event occurs", func() {
//				var desiredLRP *models.DesiredLRP
//
//				BeforeEach(func() {
//					routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
//					desiredLRP = &models.DesiredLRP{
//						Action: models.WrapAction(&models.RunAction{
//							User: "me",
//							Path: "ls",
//						}),
//						Domain:      "tests",
//						ProcessGuid: expectedProcessGuid,
//						Ports:       []uint32{expectedContainerPort},
//						Routes:      &routes,
//						LogGuid:     logGuid,
//					}
//				})
//
//				JustBeforeEach(func() {
//					fakeTable.SetRoutesReturns(dummyMessagesToEmit)
//
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewDesiredLRPCreatedEvent(desiredLRP)}))
//				})
//
//				It("should set the routes on the table", func() {
//					Eventually(fakeTable.SetRoutesCallCount).Should(Equal(1))
//
//					key, routes, _ := fakeTable.SetRoutesArgsForCall(0)
//					Expect(key).To(Equal(expectedRoutingKey))
//					Expect(routes).To(ConsistOf(
//						routingtable.Route{
//							Hostname:        expectedRoutes[0],
//							LogGuid:         logGuid,
//							RouteServiceUrl: expectedRouteServiceUrl,
//						},
//						routingtable.Route{
//							Hostname:        expectedRoutes[1],
//							LogGuid:         logGuid,
//							RouteServiceUrl: expectedRouteServiceUrl,
//						},
//					))
//				})
//
//				It("sends a 'routes registered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesRegistered")
//					}).Should(BeEquivalentTo(2))
//				})
//
//				It("sends a 'routes unregistered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesUnregistered")
//					}).Should(BeEquivalentTo(0))
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(2))
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				Context("when there are diego ssh-keys on the route", func() {
//					var (
//						foundRoutes bool
//					)
//
//					BeforeEach(func() {
//						diegoSSHInfo := json.RawMessage([]byte(`{"ssh-key": "ssh-value"}`))
//
//						routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
//						routes["diego-ssh"] = &diegoSSHInfo
//
//						desiredLRP.Routes = &routes
//					})
//
//					It("does not log them", func() {
//						Eventually(fakeTable.SetRoutesCallCount).Should(Equal(1))
//						logs := logger.Logs()
//
//						for _, log := range logs {
//							if log.Data["routes"] != nil {
//								Expect(log.Data["routes"]).ToNot(HaveKey("diego-ssh"))
//								Expect(log.Data["routes"]).To(HaveKey("cf-router"))
//								foundRoutes = true
//							}
//						}
//						if !foundRoutes {
//							Fail("Expected to find diego-ssh routes on desiredLRP")
//						}
//
//						Expect(len(*desiredLRP.Routes)).To(Equal(2))
//					})
//				})
//
//				Context("when there is a route service binding to only one hostname for a route", func() {
//					BeforeEach(func() {
//						cfRoute1 := cfroutes.CFRoute{
//							Hostnames:       []string{"route-1"},
//							Port:            expectedContainerPort,
//							RouteServiceUrl: expectedRouteServiceUrl,
//						}
//						cfRoute2 := cfroutes.CFRoute{
//							Hostnames: []string{"route-2"},
//							Port:      expectedContainerPort,
//						}
//						routes := cfroutes.CFRoutes{cfRoute1, cfRoute2}.RoutingInfo()
//						desiredLRP.Routes = &routes
//					})
//					It("registers all of the routes on the table", func() {
//						Eventually(fakeTable.SetRoutesCallCount).Should(Equal(1))
//
//						key, routes, _ := fakeTable.SetRoutesArgsForCall(0)
//						Expect(key).To(Equal(expectedRoutingKey))
//						Expect(routes).To(ConsistOf(
//							routingtable.Route{
//								Hostname:        "route-1",
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname: "route-2",
//								LogGuid:  logGuid,
//							},
//						))
//					})
//
//					It("emits whatever the table tells it to emit", func() {
//						Eventually(natsEmitter.EmitCallCount).Should(Equal(2))
//
//						messagesToEmit := natsEmitter.EmitArgsForCall(1)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//					})
//				})
//
//				Context("when there are multiple CF routes", func() {
//					BeforeEach(func() {
//						routes := cfroutes.CFRoutes{expectedCFRoute, expectedAdditionalCFRoute}.RoutingInfo()
//						desiredLRP.Routes = &routes
//					})
//
//					It("registers all of the routes on the table", func() {
//						Eventually(fakeTable.SetRoutesCallCount).Should(Equal(2))
//
//						key1, routes1, _ := fakeTable.SetRoutesArgsForCall(0)
//						key2, routes2, _ := fakeTable.SetRoutesArgsForCall(1)
//						var routes = []routingtable.Route{}
//						routes = append(routes, routes1...)
//						routes = append(routes, routes2...)
//
//						Expect([]routingtable.RoutingKey{key1, key2}).To(ConsistOf(expectedRoutingKey, expectedAdditionalRoutingKey))
//						Expect(routes).To(ConsistOf(
//							routingtable.Route{
//								Hostname:        expectedRoutes[0],
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname:        expectedRoutes[1],
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname: expectedAdditionalRoutes[0],
//								LogGuid:  logGuid,
//							},
//							routingtable.Route{
//								Hostname: expectedAdditionalRoutes[1],
//								LogGuid:  logGuid,
//							},
//						))
//					})
//
//					It("emits whatever the table tells it to emit", func() {
//						Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//						messagesToEmit := natsEmitter.EmitArgsForCall(1)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//
//						messagesToEmit = natsEmitter.EmitArgsForCall(2)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//					})
//				})
//			})
//
//			Context("when a change event occurs", func() {
//				var originalDesiredLRP, changedDesiredLRP *models.DesiredLRP
//
//				BeforeEach(func() {
//					fakeTable.SetRoutesReturns(dummyMessagesToEmit)
//					routes := cfroutes.CFRoutes{{Hostnames: expectedRoutes, Port: expectedContainerPort}}.RoutingInfo()
//
//					originalDesiredLRP = &models.DesiredLRP{
//						Action: models.WrapAction(&models.RunAction{
//							User: "me",
//							Path: "ls",
//						}),
//						Domain:      "tests",
//						ProcessGuid: expectedProcessGuid,
//						LogGuid:     logGuid,
//						Routes:      &routes,
//						Instances:   3,
//					}
//					changedDesiredLRP = &models.DesiredLRP{
//						Action: models.WrapAction(&models.RunAction{
//							User: "me",
//							Path: "ls",
//						}),
//						Domain:          "tests",
//						ProcessGuid:     expectedProcessGuid,
//						LogGuid:         logGuid,
//						Routes:          &routes,
//						ModificationTag: &models.ModificationTag{Epoch: "abcd", Index: 1},
//						Instances:       3,
//					}
//				})
//
//				JustBeforeEach(func() {
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewDesiredLRPChangedEvent(
//						originalDesiredLRP,
//						changedDesiredLRP,
//					)}))
//				})
//
//				Context("when scaling down the number of LRP instances", func() {
//					BeforeEach(func() {
//						changedDesiredLRP.Instances = 1
//
//						fakeTable.EndpointsForIndexStub = func(key routingtable.RoutingKey, index int32) []routingtable.Endpoint {
//							endpoint := routingtable.Endpoint{
//								InstanceGuid:  fmt.Sprintf("instance-guid-%d", index),
//								Index:         index,
//								Host:          fmt.Sprintf("1.1.1.%d", index),
//								Domain:        "domain",
//								Port:          expectedExternalPort,
//								ContainerPort: expectedContainerPort,
//								Evacuating:    false,
//							}
//
//							return []routingtable.Endpoint{endpoint}
//						}
//					})
//
//					It("removes route endpoints for instances that are no longer desired", func() {
//						Eventually(fakeTable.RemoveEndpointCallCount).Should(Equal(2))
//					})
//				})
//
//				It("should set the routes on the table", func() {
//					Eventually(fakeTable.SetRoutesCallCount).Should(Equal(1))
//					key, routes, _ := fakeTable.SetRoutesArgsForCall(0)
//					Expect(key).To(Equal(expectedRoutingKey))
//					Expect(routes).To(ConsistOf(
//						routingtable.Route{
//							Hostname: expectedRoutes[0],
//							LogGuid:  logGuid,
//						},
//						routingtable.Route{
//							Hostname: expectedRoutes[1],
//							LogGuid:  logGuid,
//						},
//					))
//				})
//
//				It("sends a 'routes registered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesRegistered")
//					}).Should(BeEquivalentTo(2))
//				})
//
//				It("sends a 'routes unregistered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesUnregistered")
//					}).Should(BeEquivalentTo(0))
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(2))
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				Context("when there are diego ssh-keys on the route", func() {
//					var foundRoutes bool
//
//					BeforeEach(func() {
//						diegoSSHInfo := json.RawMessage([]byte(`{"ssh-key": "ssh-value"}`))
//
//						routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
//						routes["diego-ssh"] = &diegoSSHInfo
//
//						changedDesiredLRP.Routes = &routes
//					})
//
//					It("does not log them", func() {
//						Eventually(fakeTable.SetRoutesCallCount).Should(Equal(1))
//						logs := logger.Logs()
//
//						for _, log := range logs {
//							if after, ok := log.Data["after"]; ok {
//								afterData := after.(map[string]interface{})
//
//								if afterData["routes"] != nil {
//									Expect(afterData["routes"]).ToNot(HaveKey("diego-ssh"))
//									Expect(afterData["routes"]).To(HaveKey("cf-router"))
//									foundRoutes = true
//								}
//							}
//						}
//						if !foundRoutes {
//							Fail("Expected to find diego-ssh routes on desiredLRP")
//						}
//
//						Expect(len(*changedDesiredLRP.Routes)).To(Equal(2))
//					})
//				})
//
//				Context("when CF routes are added without an associated container port", func() {
//					BeforeEach(func() {
//						routes := cfroutes.CFRoutes{expectedCFRoute, expectedAdditionalCFRoute}.RoutingInfo()
//						changedDesiredLRP.Routes = &routes
//					})
//
//					It("registers all of the routes associated with a port on the table", func() {
//						Eventually(fakeTable.SetRoutesCallCount).Should(Equal(2))
//
//						key1, routes1, _ := fakeTable.SetRoutesArgsForCall(0)
//						key2, routes2, _ := fakeTable.SetRoutesArgsForCall(1)
//						var routes = []routingtable.Route{}
//						routes = append(routes, routes1...)
//						routes = append(routes, routes2...)
//
//						Expect([]routingtable.RoutingKey{key1, key2}).To(ConsistOf(expectedRoutingKey, expectedAdditionalRoutingKey))
//						Expect(routes).To(ConsistOf(
//							routingtable.Route{
//								Hostname:        expectedRoutes[0],
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname:        expectedRoutes[1],
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname: expectedAdditionalRoutes[0],
//								LogGuid:  logGuid,
//							},
//							routingtable.Route{
//								Hostname: expectedAdditionalRoutes[1],
//								LogGuid:  logGuid,
//							},
//						))
//					})
//
//					It("emits whatever the table tells it to emit", func() {
//						Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//						messagesToEmit := natsEmitter.EmitArgsForCall(2)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//					})
//				})
//
//				Context("when CF routes and container ports are added", func() {
//					BeforeEach(func() {
//						routes := cfroutes.CFRoutes{expectedCFRoute, expectedAdditionalCFRoute}.RoutingInfo()
//						changedDesiredLRP.Routes = &routes
//					})
//
//					It("registers all of the routes on the table", func() {
//						Eventually(fakeTable.SetRoutesCallCount).Should(Equal(2))
//
//						key1, routes1, _ := fakeTable.SetRoutesArgsForCall(0)
//						key2, routes2, _ := fakeTable.SetRoutesArgsForCall(1)
//						var routes = []routingtable.Route{}
//						routes = append(routes, routes1...)
//						routes = append(routes, routes2...)
//
//						Expect([]routingtable.RoutingKey{key1, key2}).To(ConsistOf(expectedRoutingKey, expectedAdditionalRoutingKey))
//						Expect(routes).To(ConsistOf(
//							routingtable.Route{
//								Hostname:        expectedRoutes[0],
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname:        expectedRoutes[1],
//								LogGuid:         logGuid,
//								RouteServiceUrl: expectedRouteServiceUrl,
//							},
//							routingtable.Route{
//								Hostname: expectedAdditionalRoutes[0],
//								LogGuid:  logGuid,
//							},
//							routingtable.Route{
//								Hostname: expectedAdditionalRoutes[1],
//								LogGuid:  logGuid,
//							},
//						))
//					})
//
//					It("emits whatever the table tells it to emit", func() {
//						Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//						messagesToEmit := natsEmitter.EmitArgsForCall(1)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//
//						messagesToEmit = natsEmitter.EmitArgsForCall(2)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//					})
//				})
//
//				Context("when CF routes are removed", func() {
//					BeforeEach(func() {
//						routes := cfroutes.CFRoutes{}.RoutingInfo()
//						changedDesiredLRP.Routes = &routes
//
//						fakeTable.SetRoutesReturns(routingtable.MessagesToEmit{})
//						fakeTable.RemoveRoutesReturns(dummyMessagesToEmit)
//					})
//
//					It("deletes the routes for the missng key", func() {
//						Eventually(fakeTable.RemoveRoutesCallCount).Should(Equal(1))
//
//						key, modTag := fakeTable.RemoveRoutesArgsForCall(0)
//						Expect(key).To(Equal(expectedRoutingKey))
//						Expect(modTag).To(Equal(changedDesiredLRP.ModificationTag))
//					})
//
//					It("emits whatever the table tells it to emit", func() {
//						Eventually(natsEmitter.EmitCallCount).Should(Equal(2))
//
//						messagesToEmit := natsEmitter.EmitArgsForCall(1)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//					})
//				})
//			})
//
//			Context("when a delete event occurs", func() {
//				var desiredLRP *models.DesiredLRP
//
//				BeforeEach(func() {
//					fakeTable.RemoveRoutesReturns(dummyMessagesToEmit)
//					routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
//					desiredLRP = &models.DesiredLRP{
//						Action: models.WrapAction(&models.RunAction{
//							User: "me",
//							Path: "ls",
//						}),
//						Domain:          "tests",
//						ProcessGuid:     expectedProcessGuid,
//						Ports:           []uint32{expectedContainerPort},
//						Routes:          &routes,
//						LogGuid:         logGuid,
//						ModificationTag: &models.ModificationTag{Epoch: "defg", Index: 2},
//					}
//				})
//
//				JustBeforeEach(func() {
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewDesiredLRPRemovedEvent(desiredLRP)}))
//				})
//
//				It("should remove the routes from the table", func() {
//					Eventually(fakeTable.RemoveRoutesCallCount).Should(Equal(1))
//					key, modTag := fakeTable.RemoveRoutesArgsForCall(0)
//					Expect(key).To(Equal(expectedRoutingKey))
//					Expect(modTag).To(Equal(desiredLRP.ModificationTag))
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(2))
//
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				Context("when there are diego ssh-keys on the route", func() {
//					var (
//						foundRoutes bool
//					)
//
//					BeforeEach(func() {
//						diegoSSHInfo := json.RawMessage([]byte(`{"ssh-key": "ssh-value"}`))
//
//						routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
//						routes["diego-ssh"] = &diegoSSHInfo
//
//						desiredLRP.Routes = &routes
//					})
//
//					It("does not log them", func() {
//						Eventually(fakeTable.RemoveRoutesCallCount).Should(Equal(1))
//						logs := logger.Logs()
//
//						for _, log := range logs {
//							if log.Data["routes"] != nil {
//								Expect(log.Data["routes"]).ToNot(HaveKey("diego-ssh"))
//								Expect(log.Data["routes"]).To(HaveKey("cf-router"))
//								foundRoutes = true
//							}
//						}
//						if !foundRoutes {
//							Fail("Expected to find diego-ssh routes on desiredLRP")
//						}
//
//						Expect(len(*desiredLRP.Routes)).To(Equal(2))
//					})
//				})
//
//				Context("when there are multiple CF routes", func() {
//					BeforeEach(func() {
//						routes := cfroutes.CFRoutes{expectedCFRoute, expectedAdditionalCFRoute}.RoutingInfo()
//						desiredLRP.Routes = &routes
//					})
//
//					It("should remove the routes from the table", func() {
//						Eventually(fakeTable.RemoveRoutesCallCount).Should(Equal(2))
//
//						key, modTag := fakeTable.RemoveRoutesArgsForCall(0)
//						Expect(key).To(Equal(expectedRoutingKey))
//						Expect(modTag).To(Equal(desiredLRP.ModificationTag))
//
//						key, modTag = fakeTable.RemoveRoutesArgsForCall(1)
//						Expect(key).To(Equal(expectedAdditionalRoutingKey))
//
//						key, modTag = fakeTable.RemoveRoutesArgsForCall(0)
//						Expect(key).To(Equal(expectedRoutingKey))
//						Expect(modTag).To(Equal(desiredLRP.ModificationTag))
//					})
//
//					It("emits whatever the table tells it to emit", func() {
//						Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//						messagesToEmit := natsEmitter.EmitArgsForCall(1)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//
//						messagesToEmit = natsEmitter.EmitArgsForCall(2)
//						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//					})
//				})
//			})
//		}
//
//		Context("when the cell id is set", func() {
//			BeforeEach(func() {
//				cellID = "cell-a"
//			})
//
//			behaveAsDesired()
//		})
//
//		Context("when the cell id is not set", func() {
//			BeforeEach(func() {
//				cellID = ""
//			})
//
//			behaveAsDesired()
//		})
//	})
//
//	Describe("Actual LRP changes", func() {
//		JustBeforeEach(func() {
//			syncEvents.Sync <- struct{}{}
//			Eventually(natsEmitter.EmitCallCount).ShouldNot(Equal(0))
//		})
//
//		Context("when a create event occurs", func() {
//			var (
//				actualLRPGroup       *models.ActualLRPGroup
//				actualLRP            *models.ActualLRP
//				actualLRPRoutingInfo *routingtable.ActualLRPRoutingInfo
//			)
//
//			Context("when the resulting LRP is in the RUNNING state", func() {
//				BeforeEach(func() {
//					actualLRP = &models.ActualLRP{
//						ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//						ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//						ActualLRPNetInfo: models.NewActualLRPNetInfo(
//							expectedHost,
//							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
//							models.NewPortMapping(expectedExternalPort, expectedAdditionalContainerPort),
//						),
//						State: models.ActualLRPStateRunning,
//					}
//
//					actualLRPGroup = &models.ActualLRPGroup{
//						Instance: actualLRP,
//					}
//
//					actualLRPRoutingInfo = &routingtable.ActualLRPRoutingInfo{
//						ActualLRP:  actualLRP,
//						Evacuating: false,
//					}
//				})
//
//				JustBeforeEach(func() {
//					fakeTable.AddEndpointReturns(dummyMessagesToEmit)
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPCreatedEvent(actualLRPGroup)}))
//				})
//
//				It("should log the net info", func() {
//					Eventually(logger).Should(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedExternalPort,
//						),
//					))
//				})
//
//				It("should add/update the endpoints on the table", func() {
//					Eventually(fakeTable.AddEndpointCallCount).Should(Equal(2))
//
//					keys := routingtable.RoutingKeysFromActual(actualLRP)
//					endpoints, err := routingtable.EndpointsFromActual(actualLRPRoutingInfo)
//					Expect(err).NotTo(HaveOccurred())
//
//					key, endpoint := fakeTable.AddEndpointArgsForCall(0)
//					Expect(keys).To(ContainElement(key))
//					Expect(endpoint).To(Equal(endpoints[key.ContainerPort]))
//
//					key, endpoint = fakeTable.AddEndpointArgsForCall(1)
//					Expect(keys).To(ContainElement(key))
//					Expect(endpoint).To(Equal(endpoints[key.ContainerPort]))
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				It("sends a 'routes registered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesRegistered")
//					}).Should(BeEquivalentTo(4))
//				})
//
//				It("sends a 'routes unregistered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesUnregistered")
//					}).Should(BeEquivalentTo(0))
//				})
//
//				Context("when a cell id is set", func() {
//					BeforeEach(func() {
//						cellID = "cell-a"
//					})
//
//					Context("and the event cell-id matches", func() {
//						BeforeEach(func() {
//							actualLRP.ActualLRPInstanceKey.CellId = "cell-a"
//						})
//
//						It("should add/update the endpoints on the table", func() {
//							Eventually(fakeTable.AddEndpointCallCount).Should(Equal(2))
//						})
//					})
//
//					Context("and the event cell-id does not match", func() {
//						BeforeEach(func() {
//							actualLRP.ActualLRPInstanceKey.CellId = "cell-b"
//						})
//
//						It("should add/update the endpoints on the table", func() {
//							Consistently(fakeTable.AddEndpointCallCount).Should(Equal(0))
//						})
//					})
//				})
//			})
//
//			Context("when the resulting LRP is not in the RUNNING state", func() {
//				JustBeforeEach(func() {
//					actualLRP = &models.ActualLRP{
//						ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//						ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//						ActualLRPNetInfo: models.NewActualLRPNetInfo(
//							expectedHost,
//							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
//							models.NewPortMapping(expectedExternalPort, expectedAdditionalContainerPort),
//						),
//						State: models.ActualLRPStateUnclaimed,
//					}
//
//					actualLRPGroup = &models.ActualLRPGroup{
//						Instance: actualLRP,
//					}
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPCreatedEvent(actualLRPGroup)}))
//				})
//
//				It("should NOT log the net info", func() {
//					Consistently(logger).ShouldNot(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedExternalPort,
//						),
//					))
//				})
//
//				It("doesn't add/update the endpoint on the table", func() {
//					Consistently(fakeTable.AddEndpointCallCount).Should(Equal(0))
//				})
//
//				It("doesn't emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(1))
//				})
//			})
//		})
//
//		Context("when a change event occurs", func() {
//			Context("when the resulting LRP is in the RUNNING state", func() {
//				var (
//					afterActualLRP, beforeActualLRP *models.ActualLRPGroup
//				)
//
//				BeforeEach(func() {
//					fakeTable.AddEndpointReturns(dummyMessagesToEmit)
//
//					beforeActualLRP = &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//							State:                models.ActualLRPStateClaimed,
//						},
//					}
//					afterActualLRP = &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//							ActualLRPNetInfo: models.NewActualLRPNetInfo(
//								expectedHost,
//								models.NewPortMapping(expectedExternalPort, expectedContainerPort),
//								models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
//							),
//							State: models.ActualLRPStateRunning,
//						},
//					}
//				})
//
//				JustBeforeEach(func() {
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPChangedEvent(beforeActualLRP, afterActualLRP)}))
//				})
//
//				It("should log the new net info", func() {
//					Eventually(logger).Should(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedAdditionalExternalPort,
//						),
//					))
//				})
//
//				It("should add/update the endpoint on the table", func() {
//					Eventually(fakeTable.AddEndpointCallCount).Should(Equal(2))
//
//					// Verify the arguments that were passed to AddEndpoint independent of which call was made first.
//					type endpointArgs struct {
//						key      routingtable.RoutingKey
//						endpoint routingtable.Endpoint
//					}
//					args := make([]endpointArgs, 2)
//					key, endpoint := fakeTable.AddEndpointArgsForCall(0)
//					args[0] = endpointArgs{key, endpoint}
//					key, endpoint = fakeTable.AddEndpointArgsForCall(1)
//					args[1] = endpointArgs{key, endpoint}
//
//					Expect(args).To(ConsistOf([]endpointArgs{
//						endpointArgs{expectedRoutingKey, routingtable.Endpoint{
//							InstanceGuid:  expectedInstanceGuid,
//							Index:         expectedIndex,
//							Host:          expectedHost,
//							Domain:        expectedDomain,
//							Port:          expectedExternalPort,
//							ContainerPort: expectedContainerPort,
//						}},
//						endpointArgs{expectedAdditionalRoutingKey, routingtable.Endpoint{
//							InstanceGuid:  expectedInstanceGuid,
//							Index:         expectedIndex,
//							Host:          expectedHost,
//							Domain:        expectedDomain,
//							Port:          expectedAdditionalExternalPort,
//							ContainerPort: expectedAdditionalContainerPort,
//						}},
//					}))
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				It("sends a 'routes registered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesRegistered")
//					}).Should(BeEquivalentTo(4))
//				})
//
//				It("sends a 'routes unregistered' metric", func() {
//					Eventually(func() uint64 {
//						return fakeMetricSender.GetCounter("RoutesUnregistered")
//					}).Should(BeEquivalentTo(0))
//				})
//
//				Context("when a cell id is set", func() {
//					BeforeEach(func() {
//						cellID = "cell-a"
//					})
//
//					Context("and the event cell-id matches", func() {
//						BeforeEach(func() {
//							afterActualLRP.Instance.ActualLRPInstanceKey.CellId = "cell-a"
//						})
//
//						It("should add/update the endpoints on the table", func() {
//							Eventually(fakeTable.AddEndpointCallCount).Should(Equal(2))
//						})
//					})
//
//					Context("and the event cell-id does not match", func() {
//						BeforeEach(func() {
//							afterActualLRP.Instance.ActualLRPInstanceKey.CellId = "cell-b"
//						})
//
//						It("should add/update the endpoints on the table", func() {
//							Consistently(fakeTable.AddEndpointCallCount).Should(Equal(0))
//						})
//					})
//				})
//			})
//
//			Context("when the resulting LRP transitions away from the RUNNING state", func() {
//				var (
//					beforeActualLRP, afterActualLRP *models.ActualLRPGroup
//				)
//
//				BeforeEach(func() {
//					beforeActualLRP = &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//							ActualLRPNetInfo: models.NewActualLRPNetInfo(
//								expectedHost,
//								models.NewPortMapping(expectedExternalPort, expectedContainerPort),
//								models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
//							),
//							State: models.ActualLRPStateRunning,
//						},
//					}
//					afterActualLRP = &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey: models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							State:        models.ActualLRPStateUnclaimed,
//						},
//					}
//
//				})
//
//				JustBeforeEach(func() {
//					fakeTable.RemoveEndpointReturns(dummyMessagesToEmit)
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPChangedEvent(beforeActualLRP, afterActualLRP)}))
//				})
//
//				It("should log the previous net info", func() {
//					Eventually(logger).Should(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedAdditionalExternalPort,
//						),
//					))
//				})
//
//				It("should remove the endpoint from the table", func() {
//					Eventually(fakeTable.RemoveEndpointCallCount).Should(Equal(2))
//
//					key, endpoint := fakeTable.RemoveEndpointArgsForCall(0)
//					Expect(key).To(Equal(expectedRoutingKey))
//					Expect(endpoint).To(Equal(routingtable.Endpoint{
//						InstanceGuid:  expectedInstanceGuid,
//						Index:         expectedIndex,
//						Host:          expectedHost,
//						Domain:        expectedDomain,
//						Port:          expectedExternalPort,
//						ContainerPort: expectedContainerPort,
//					}))
//
//					key, endpoint = fakeTable.RemoveEndpointArgsForCall(1)
//					Expect(key).To(Equal(expectedAdditionalRoutingKey))
//					Expect(endpoint).To(Equal(routingtable.Endpoint{
//						InstanceGuid:  expectedInstanceGuid,
//						Index:         expectedIndex,
//						Host:          expectedHost,
//						Domain:        expectedDomain,
//						Port:          expectedAdditionalExternalPort,
//						ContainerPort: expectedAdditionalContainerPort,
//					}))
//
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				Context("when a cell id is set", func() {
//					BeforeEach(func() {
//						cellID = "cell-a"
//					})
//
//					Context("and the event cell-id matches", func() {
//						BeforeEach(func() {
//							beforeActualLRP.Instance.CellId = "cell-a"
//						})
//
//						It("should remove the endpoint from the table", func() {
//							Eventually(fakeTable.RemoveEndpointCallCount).Should(Equal(2))
//						})
//					})
//
//					Context("and the event cell-id does not match", func() {
//						BeforeEach(func() {
//							beforeActualLRP.Instance.CellId = "cell-b"
//						})
//
//						It("should not remove the endpoint from the table", func() {
//							Consistently(fakeTable.RemoveEndpointCallCount).Should(Equal(0))
//						})
//					})
//				})
//			})
//
//			Context("when the endpoint neither starts nor ends in the RUNNING state", func() {
//				JustBeforeEach(func() {
//					beforeActualLRP := &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey: models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							State:        models.ActualLRPStateUnclaimed,
//						},
//					}
//					afterActualLRP := &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//							State:                models.ActualLRPStateClaimed,
//						},
//					}
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPChangedEvent(beforeActualLRP, afterActualLRP)}))
//				})
//
//				It("should NOT log the net info", func() {
//					Consistently(logger).ShouldNot(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedExternalPort,
//						),
//					))
//				})
//
//				It("should not remove the endpoint", func() {
//					Consistently(fakeTable.RemoveEndpointCallCount).Should(BeZero())
//				})
//
//				It("should not add or update the endpoint", func() {
//					Consistently(fakeTable.AddEndpointCallCount).Should(BeZero())
//				})
//
//				It("should not emit anything", func() {
//					Consistently(natsEmitter.EmitCallCount).Should(Equal(1))
//				})
//			})
//
//		})
//
//		Context("when a delete event occurs", func() {
//			Context("when the actual is in the RUNNING state", func() {
//				var (
//					actualLRP *models.ActualLRPGroup
//				)
//
//				BeforeEach(func() {
//					fakeTable.RemoveEndpointReturns(dummyMessagesToEmit)
//
//					actualLRP = &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGuid, "cell-id"),
//							ActualLRPNetInfo: models.NewActualLRPNetInfo(
//								expectedHost,
//								models.NewPortMapping(expectedExternalPort, expectedContainerPort),
//								models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
//							),
//							State: models.ActualLRPStateRunning,
//						},
//					}
//				})
//
//				JustBeforeEach(func() {
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRP)}))
//				})
//
//				It("should log the previous net info", func() {
//					Eventually(logger).Should(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedAdditionalExternalPort,
//						),
//					))
//				})
//
//				It("should remove the endpoint from the table", func() {
//					Eventually(fakeTable.RemoveEndpointCallCount).Should(Equal(2))
//
//					key, endpoint := fakeTable.RemoveEndpointArgsForCall(0)
//					Expect(key).To(Equal(expectedRoutingKey))
//					Expect(endpoint).To(Equal(routingtable.Endpoint{
//						InstanceGuid:  expectedInstanceGuid,
//						Index:         expectedIndex,
//						Host:          expectedHost,
//						Domain:        expectedDomain,
//						Port:          expectedExternalPort,
//						ContainerPort: expectedContainerPort,
//					}))
//
//					key, endpoint = fakeTable.RemoveEndpointArgsForCall(1)
//					Expect(key).To(Equal(expectedAdditionalRoutingKey))
//					Expect(endpoint).To(Equal(routingtable.Endpoint{
//						InstanceGuid:  expectedInstanceGuid,
//						Index:         expectedIndex,
//						Host:          expectedHost,
//						Domain:        expectedDomain,
//						Port:          expectedAdditionalExternalPort,
//						ContainerPort: expectedAdditionalContainerPort,
//					}))
//
//				})
//
//				It("should emit whatever the table tells it to emit", func() {
//					Eventually(natsEmitter.EmitCallCount).Should(Equal(3))
//
//					messagesToEmit := natsEmitter.EmitArgsForCall(1)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//
//					messagesToEmit = natsEmitter.EmitArgsForCall(2)
//					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
//				})
//
//				Context("when a cell id is set", func() {
//					BeforeEach(func() {
//						cellID = "cell-a"
//					})
//
//					Context("and the event cell-id matches", func() {
//						BeforeEach(func() {
//							actualLRP.Instance.CellId = "cell-a"
//						})
//
//						It("should remove the endpoint from the table", func() {
//							Eventually(fakeTable.RemoveEndpointCallCount).Should(Equal(2))
//						})
//					})
//
//					Context("and the event cell-id does not match", func() {
//						BeforeEach(func() {
//							actualLRP.Instance.CellId = "cell-b"
//						})
//
//						It("should not remove the endpoint from the table", func() {
//							Consistently(fakeTable.RemoveEndpointCallCount).Should(Equal(0))
//						})
//					})
//				})
//			})
//
//			Context("when the actual is not in the RUNNING state", func() {
//				JustBeforeEach(func() {
//					actualLRP := &models.ActualLRPGroup{
//						Instance: &models.ActualLRP{
//							ActualLRPKey: models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
//							State:        models.ActualLRPStateCrashed,
//						},
//					}
//
//					Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRP)}))
//				})
//
//				It("should NOT log the net info", func() {
//					Consistently(logger).ShouldNot(Say(
//						fmt.Sprintf(
//							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
//							expectedHost,
//							expectedContainerPort,
//							expectedExternalPort,
//							expectedAdditionalContainerPort,
//							expectedExternalPort,
//						),
//					))
//				})
//
//				It("doesn't remove the endpoint from the table", func() {
//					Consistently(fakeTable.RemoveEndpointCallCount).Should(Equal(0))
//				})
//
//				It("doesn't emit", func() {
//					Consistently(natsEmitter.EmitCallCount).Should(Equal(1))
//				})
//			})
//		})
//	})
//
//	Describe("Unrecognized events", func() {
//		JustBeforeEach(func() {
//			syncEvents.Sync <- struct{}{}
//			Eventually(natsEmitter.EmitCallCount).Should(Equal(1))
//			Eventually(eventCh).Should(BeSent(EventHolder{&unrecognizedEvent{}}))
//		})
//
//		It("does not emit any more messages", func() {
//			Consistently(natsEmitter.EmitCallCount).Should(Equal(1))
//		})
//	})
//
//	Context("when the event source returns an error", func() {
//		var subscribeErr error
//
//		BeforeEach(func() {
//			subscribeErr = errors.New("subscribe-error")
//
//			bbsClient.SubscribeToEventsStub = func(lager.Logger) (events.EventSource, error) {
//				if bbsClient.SubscribeToEventsCallCount() == 1 {
//					return eventSource, nil
//				}
//				return nil, subscribeErr
//			}
//
//			eventSource.NextStub = func() (models.Event, error) {
//				return nil, errors.New("next-error")
//			}
//		})
//
//		JustBeforeEach(func() {
//			syncEvents.Sync <- struct{}{}
//		})
//
//		It("re-subscribes", func() {
//			Eventually(bbsClient.SubscribeToEventsCallCount, 2*time.Second).Should(BeNumerically(">", 5))
//		})
//
//		It("does not exit", func() {
//			Consistently(process.Wait()).ShouldNot(Receive())
//		})
//
//		It("closes any unused connections", func() {
//			Eventually(eventSource.CloseCallCount, 2*time.Second).Should(Equal(1))
//		})
//	})
//
//	Describe("interrupting the process", func() {
//		It("should be possible to SIGINT the route emitter", func() {
//			process.Signal(os.Interrupt)
//			Eventually(process.Wait()).Should(Receive())
//		})
//	})
//
//	Describe("Sync Events", func() {
//		Context("Emit", func() {
//			JustBeforeEach(func() {
//				fakeTable.MessagesToEmitReturns(dummyMessagesToEmit)
//				fakeTable.RouteCountReturns(123)
//				syncEvents.Emit <- struct{}{}
//			})
//
//			It("emits", func() {
//				Eventually(natsEmitter.EmitCallCount).Should(Equal(1))
//				Expect(natsEmitter.EmitArgsForCall(0)).To(Equal(dummyMessagesToEmit))
//			})
//
//			It("sends a 'routes total' metric", func() {
//				Eventually(func() float64 {
//					return fakeMetricSender.GetValue("RoutesTotal").Value
//				}, 2).Should(BeEquivalentTo(123))
//			})
//
//			It("sends a 'synced routes' metric", func() {
//				Eventually(func() uint64 {
//					return fakeMetricSender.GetCounter("RoutesSynced")
//				}, 2).Should(BeEquivalentTo(2))
//			})
//		})
//
//		Context("Begin & End events", func() {
//			currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
//			hostname1 := "foo.example.com"
//			hostname2 := "bar.example.com"
//			hostname3 := "baz.example.com"
//			endpoint1 := routingtable.Endpoint{InstanceGuid: "ig-1", Host: "1.1.1.1", Index: 0, Port: 11, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
//			endpoint2 := routingtable.Endpoint{InstanceGuid: "ig-2", Host: "2.2.2.2", Index: 0, Port: 22, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
//			endpoint3 := routingtable.Endpoint{InstanceGuid: "ig-3", Host: "2.2.2.2", Index: 1, Port: 23, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
//
//			schedulingInfo1 := &models.DesiredLRPSchedulingInfo{
//				DesiredLRPKey: models.NewDesiredLRPKey("pg-1", "tests", "lg1"),
//				Routes: cfroutes.CFRoutes{
//					cfroutes.CFRoute{
//						Hostnames:       []string{hostname1},
//						Port:            8080,
//						RouteServiceUrl: "https://rs.example.com",
//					},
//				}.RoutingInfo(),
//				Instances: 1,
//			}
//
//			schedulingInfo2 := &models.DesiredLRPSchedulingInfo{
//				DesiredLRPKey: models.NewDesiredLRPKey("pg-2", "tests", "lg2"),
//				Routes: cfroutes.CFRoutes{
//					cfroutes.CFRoute{
//						Hostnames: []string{hostname2},
//						Port:      8080,
//					},
//				}.RoutingInfo(),
//				Instances: 1,
//			}
//
//			schedulingInfo3 := &models.DesiredLRPSchedulingInfo{
//				DesiredLRPKey: models.NewDesiredLRPKey("pg-3", "tests", "lg3"),
//				Routes: cfroutes.CFRoutes{
//					cfroutes.CFRoute{
//						Hostnames: []string{hostname3},
//						Port:      8080,
//					},
//				}.RoutingInfo(),
//				Instances: 1,
//			}
//
//			actualLRPGroup1 := &models.ActualLRPGroup{
//				Instance: &models.ActualLRP{
//					ActualLRPKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
//					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGuid, "cell-id"),
//					ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
//					State:                models.ActualLRPStateRunning,
//				},
//			}
//
//			actualLRPGroup2 := &models.ActualLRPGroup{
//				Instance: &models.ActualLRP{
//					ActualLRPKey:         models.NewActualLRPKey("pg-2", 0, "domain"),
//					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint2.InstanceGuid, "cell-id"),
//					ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint2.Host, models.NewPortMapping(endpoint2.Port, endpoint2.ContainerPort)),
//					State:                models.ActualLRPStateRunning,
//				},
//			}
//
//			actualLRPGroup3 := &models.ActualLRPGroup{
//				Instance: &models.ActualLRP{
//					ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
//					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGuid, "cell-id"),
//					ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint3.Host, models.NewPortMapping(endpoint3.Port, endpoint3.ContainerPort)),
//					State:                models.ActualLRPStateRunning,
//				},
//			}
//
//			sendEvent := func() {
//				Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPRemovedEvent(actualLRPGroup1)}))
//			}
//
//			Context("when sync begins", func() {
//				JustBeforeEach(func() {
//					syncEvents.Sync <- struct{}{}
//				})
//
//				Describe("bbs events", func() {
//					var ready chan struct{}
//					var count int32
//
//					BeforeEach(func() {
//						ready = make(chan struct{})
//						count = 0
//
//						bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
//							defer GinkgoRecover()
//
//							atomic.AddInt32(&count, 1)
//							ready <- struct{}{}
//							Eventually(ready).Should(Receive())
//							return nil, nil
//						}
//					})
//
//					JustBeforeEach(func() {
//						Eventually(ready).Should(Receive())
//					})
//
//					It("caches events", func() {
//						sendEvent()
//						Consistently(fakeTable.RemoveEndpointCallCount).Should(Equal(0))
//						ready <- struct{}{}
//					})
//
//					Context("additional sync events", func() {
//						JustBeforeEach(func() {
//							syncEvents.Sync <- struct{}{}
//						})
//
//						It("ignores the sync event", func() {
//							Consistently(atomic.LoadInt32(&count)).Should(Equal(int32(1)))
//							ready <- struct{}{}
//						})
//					})
//				})
//
//				Context("when fetching actuals fails", func() {
//					var returnError int32
//
//					BeforeEach(func() {
//						returnError = 1
//
//						bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
//							if atomic.LoadInt32(&returnError) == 1 {
//								return nil, errors.New("bam")
//							}
//
//							return []*models.ActualLRPGroup{}, nil
//						}
//					})
//
//					It("should not call sync until the error resolves", func() {
//						Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
//						Consistently(fakeTable.SwapCallCount).Should(Equal(0))
//
//						atomic.StoreInt32(&returnError, 0)
//						syncEvents.Sync <- struct{}{}
//
//						Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//						Expect(bbsClient.ActualLRPGroupsCallCount()).To(Equal(2))
//					})
//				})
//
//				Context("when fetching desireds fails", func() {
//					var returnError int32
//
//					BeforeEach(func() {
//						returnError = 1
//
//						bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
//							if atomic.LoadInt32(&returnError) == 1 {
//								return nil, errors.New("bam")
//							}
//
//							return []*models.DesiredLRPSchedulingInfo{}, nil
//						}
//					})
//
//					It("should not call sync until the error resolves", func() {
//						Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount).Should(Equal(1))
//						Consistently(fakeTable.SwapCallCount).Should(Equal(0))
//
//						atomic.StoreInt32(&returnError, 0)
//						syncEvents.Sync <- struct{}{}
//
//						Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//						Expect(bbsClient.DesiredLRPSchedulingInfosCallCount()).To(Equal(2))
//					})
//				})
//			})
//
//			Context("when syncing ends", func() {
//				var ready chan struct{}
//
//				JustBeforeEach(func() {
//					syncEvents.Sync <- struct{}{}
//				})
//
//				It("swaps the tables", func() {
//					Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//				})
//
//				Context("when desired lrps are retrieved", func() {
//					BeforeEach(func() {
//						bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
//							clock.IncrementBySeconds(1)
//
//							return []*models.ActualLRPGroup{
//								actualLRPGroup1,
//								actualLRPGroup2,
//								actualLRPGroup3,
//							}, nil
//						}
//
//						ready = make(chan struct{})
//
//						bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
//							defer GinkgoRecover()
//
//							ready <- struct{}{}
//							Eventually(ready).Should(Receive())
//
//							return []*models.DesiredLRPSchedulingInfo{schedulingInfo1, schedulingInfo2}, nil
//						}
//					})
//
//					It("should emit the sync duration, and allow event processing", func() {
//						Eventually(ready).Should(Receive())
//						ready <- struct{}{}
//						Eventually(func() float64 {
//							return fakeMetricSender.GetValue("RouteEmitterSyncDuration").Value
//						}).Should(BeNumerically(">=", 100*time.Millisecond))
//
//						By("completing, events are no longer cached")
//						sendEvent()
//
//						Eventually(fakeTable.RemoveEndpointCallCount).Should(Equal(1))
//					})
//
//					Context("a table with a single routable endpoint", func() {
//						BeforeEach(func() {
//							actualLRPRoutingInfo1 := &routingtable.ActualLRPRoutingInfo{
//								ActualLRP:  actualLRPGroup1.Instance,
//								Evacuating: false,
//							}
//
//							actualLRPRoutingInfo2 := &routingtable.ActualLRPRoutingInfo{
//								ActualLRP:  actualLRPGroup2.Instance,
//								Evacuating: false,
//							}
//							tempTable := routingtable.NewTempTable(
//								routingtable.RoutesByRoutingKeyFromSchedulingInfos([]*models.DesiredLRPSchedulingInfo{schedulingInfo1, schedulingInfo2}),
//								routingtable.EndpointsByRoutingKeyFromActuals([]*routingtable.ActualLRPRoutingInfo{
//									actualLRPRoutingInfo1,
//									actualLRPRoutingInfo2,
//								},
//									map[string]*models.DesiredLRPSchedulingInfo{
//										schedulingInfo1.ProcessGuid: schedulingInfo1,
//										schedulingInfo2.ProcessGuid: schedulingInfo2}),
//							)
//
//							domains := models.NewDomainSet([]string{"domain"})
//							table = routingtable.NewNATSTable(logger)
//							table.Swap(tempTable, domains)
//						})
//
//						It("gets all the desired lrps", func() {
//							Eventually(ready).Should(Receive())
//							ready <- struct{}{}
//							Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(1))
//							_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(0)
//							Expect(filter.ProcessGuids).To(BeEmpty())
//						})
//
//						It("applies the cached events and emits", func() {
//							Eventually(ready).Should(Receive())
//							sendEvent()
//
//							Eventually(logger).Should(Say("caching-event"))
//
//							ready <- struct{}{}
//
//							Eventually(natsEmitter.EmitCallCount).Should(Equal(1))
//							Expect(natsEmitter.EmitArgsForCall(0)).To(Equal(routingtable.MessagesToEmit{
//								RegistrationMessages: []routingtable.RegistryMessage{
//									routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGuid: "lg2"}),
//								},
//								UnregistrationMessages: []routingtable.RegistryMessage{
//									routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGuid: "lg1", RouteServiceUrl: "https://rs.example.com"}),
//								},
//							}))
//						})
//					})
//
//					Context("when the cell id is set", func() {
//						var (
//							routingTable routingtable.RoutingTable
//						)
//
//						BeforeEach(func() {
//							cellID = "another-cell-id"
//							actualLRPGroup2.Instance.ActualLRPInstanceKey.CellId = cellID
//						})
//
//						Context("when the cell has actual lrps running", func() {
//							BeforeEach(func() {
//								bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
//									clock.IncrementBySeconds(1)
//
//									return []*models.ActualLRPGroup{
//										actualLRPGroup2,
//									}, nil
//								}
//							})
//
//							JustBeforeEach(func() {
//								Eventually(ready).Should(Receive())
//								ready <- struct{}{}
//
//								Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//								routingTable, _ = fakeTable.SwapArgsForCall(0)
//							})
//
//							It("does not register endpoints for lrps on other cells", func() {
//								keys := routingtable.RoutingKeysFromActual(actualLRPGroup1.Instance)
//								Expect(keys).To(HaveLen(1))
//								endpoints := routingTable.EndpointsForIndex(keys[0], 0)
//								Expect(endpoints).To(HaveLen(0))
//							})
//
//							It("registers endpoints for lrps on this cell", func() {
//								keys := routingtable.RoutingKeysFromActual(actualLRPGroup2.Instance)
//								Expect(keys).To(HaveLen(1))
//								endpoints := routingTable.EndpointsForIndex(keys[0], 0)
//								Expect(endpoints).To(HaveLen(1))
//							})
//
//							It("fetches actual lrps that match the cell id", func() {
//								Eventually(bbsClient.ActualLRPGroupsCallCount).Should(Equal(1))
//								_, filter := bbsClient.ActualLRPGroupsArgsForCall(0)
//								Expect(filter.CellID).To(Equal(cellID))
//							})
//
//							It("fetches desired lrp scheduling info that match the cell id", func() {
//								Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(1))
//								_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(0)
//								lrp, _ := actualLRPGroup2.Resolve()
//								Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))
//							})
//						})
//
//						Context("when desired lrp for the actual lrp is missing", func() {
//							BeforeEach(func() {
//								cellID = "cell-id"
//
//								bbsClient.DesiredLRPSchedulingInfosStub = func(logger lager.Logger, f models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
//									defer GinkgoRecover()
//									ready <- struct{}{}
//									Eventually(ready).Should(Receive())
//									if len(f.ProcessGuids) == 1 && f.ProcessGuids[0] == "pg-3" {
//										return []*models.DesiredLRPSchedulingInfo{schedulingInfo3}, nil
//									}
//									return []*models.DesiredLRPSchedulingInfo{schedulingInfo1}, nil
//								}
//
//								bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
//									clock.IncrementBySeconds(1)
//									return []*models.ActualLRPGroup{actualLRPGroup1}, nil
//								}
//
//								fakeTable.SetRoutesReturns(dummyMessagesToEmit)
//							})
//
//							JustBeforeEach(func() {
//								Eventually(ready).Should(Receive())
//								ready <- struct{}{}
//
//								beforeActualLRPGroup3 := &models.ActualLRPGroup{
//									Instance: &models.ActualLRP{
//										ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
//										ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGuid, "cell-id"),
//										State:                models.ActualLRPStateClaimed,
//									},
//								}
//
//								Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPChangedEvent(
//									beforeActualLRPGroup3,
//									actualLRPGroup3,
//								)}))
//							})
//
//							It("fetches the desired lrp and updates the routing table", func() {
//								Eventually(ready).Should(Receive())
//								ready <- struct{}{}
//
//								Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//								routingTable, _ = fakeTable.SwapArgsForCall(0)
//
//								Eventually(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(2))
//
//								_, filter := bbsClient.DesiredLRPSchedulingInfosArgsForCall(1)
//								lrp, _ := actualLRPGroup3.Resolve()
//
//								Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//								routingTable, _ = fakeTable.SwapArgsForCall(0)
//								Expect(filter.ProcessGuids).To(HaveLen(1))
//								Expect(filter.ProcessGuids).To(ConsistOf(lrp.ProcessGuid))
//							})
//						})
//						Context("when there are no running actual lrps on the cell", func() {
//							BeforeEach(func() {
//								bbsClient.ActualLRPGroupsStub = func(logger lager.Logger, f models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
//									clock.IncrementBySeconds(1)
//
//									ready <- struct{}{}
//									Eventually(ready).Should(Receive())
//
//									return []*models.ActualLRPGroup{}, nil
//								}
//							})
//
//							JustBeforeEach(func() {
//								Eventually(ready).Should(Receive())
//								ready <- struct{}{}
//
//								Eventually(fakeTable.SwapCallCount).Should(Equal(1))
//								routingTable, _ = fakeTable.SwapArgsForCall(0)
//							})
//
//							It("does not fetch any desired lrp scheduling info", func() {
//								Consistently(bbsClient.DesiredLRPSchedulingInfosCallCount()).Should(Equal(0))
//							})
//						})
//					})
//				})
//			})
//		})
//	})
//})
//
//type unrecognizedEvent struct{}
//
//func (u *unrecognizedEvent) EventType() string { return "unrecognized-event" }
//func (u *unrecognizedEvent) Key() string       { return "" }
//func (u *unrecognizedEvent) Reset()            {}
//func (u *unrecognizedEvent) ProtoMessage()     {}
//func (u *unrecognizedEvent) String() string    { return "unrecognized-event" }
