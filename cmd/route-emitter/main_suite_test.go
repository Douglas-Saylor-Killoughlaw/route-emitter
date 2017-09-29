package main_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/diegonats/gnatsdrunner"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	bbstestrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/consuladapter/consulrunner"
)

const heartbeatInterval = 1 * time.Second

var (
	cfgs []func(*config.RouteEmitterConfig)

	emitterPath        string
	natsPort           int
	dropsondePort      int
	healthCheckPort    int
	healthCheckAddress string

	oauthServer *ghttp.Server

	bbsPath    string
	bbsURL     *url.URL
	bbsConfig  bbsconfig.BBSConfig
	bbsRunner  *ginkgomon.Runner
	bbsProcess ifrit.Process

	routingAPIPath string

	consulRunner               *consulrunner.ClusterRunner
	gnatsdRunner               ifrit.Process
	natsClient                 diegonats.NATSClient
	bbsClient                  bbs.InternalClient
	logger                     *lagertest.TestLogger
	emitInterval, syncInterval time.Duration
	consulClusterAddress       string
	testMetricsListener        net.PacketConn
	testMetricsChan            chan interface{}

	sqlProcess        ifrit.Process
	sqlRunner         sqlrunner.SQLRunner
	bbsRunning        = false
	useLoggregatorV2  = false
	testIngressServer *testhelpers.TestIngressServer
)

func TestRouteEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(15 * time.Second)
	RunSpecs(t, "Route Emitter Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	emitter, err := gexec.Build("code.cloudfoundry.org/route-emitter/cmd/route-emitter", "-race")
	Expect(err).NotTo(HaveOccurred())

	bbs, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	routingAPI, err := gexec.Build("code.cloudfoundry.org/routing-api/cmd/routing-api", "-race")
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"emitter":     emitter,
		"bbs":         bbs,
		"routing-api": routingAPI,
	})

	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	oauthServer = startOAuthServer()

	binaries := map[string]string{}

	err := json.Unmarshal(payload, &binaries)
	Expect(err).NotTo(HaveOccurred())

	natsPort = 4001 + GinkgoParallelNode()

	emitterPath = string(binaries["emitter"])

	dbName := fmt.Sprintf("diego_%d", GinkgoParallelNode())
	sqlRunner = test_helpers.NewSQLRunner(dbName)

	consulRunner = consulrunner.NewClusterRunner(
		consulrunner.ClusterRunnerConfig{
			StartingPort: 9001 + GinkgoParallelNode()*consulrunner.PortOffsetLength,
			NumNodes:     1,
			Scheme:       "http",
		},
	)

	logger = lagertest.NewTestLogger("test")

	syncInterval = 200 * time.Millisecond
	emitInterval = time.Second

	bbsPath = string(binaries["bbs"])
	bbsPort := 13000 + GinkgoParallelNode()*2
	bbsHealthPort := bbsPort + 1
	bbsAddress := fmt.Sprintf("127.0.0.1:%d", bbsPort)
	bbsHealthAddress := fmt.Sprintf("127.0.0.1:%d", bbsHealthPort)
	routingAPIPath = string(binaries["routing-api"])

	bbsURL = &url.URL{
		Scheme: "http",
		Host:   bbsAddress,
	}

	bbsClient = bbs.NewClient(bbsURL.String())

	bbsConfig = bbsconfig.BBSConfig{
		ListenAddress:            bbsAddress,
		AdvertiseURL:             bbsURL.String(),
		AuctioneerAddress:        "http://some-address",
		DatabaseDriver:           sqlRunner.DriverName(),
		DatabaseConnectionString: sqlRunner.ConnectionString(),
		ConsulCluster:            consulRunner.ConsulCluster(),
		HealthAddress:            bbsHealthAddress,

		EncryptionConfig: encryption.EncryptionConfig{
			EncryptionKeys: map[string]string{"label": "key"},
			ActiveKeyLabel: "label",
		},
	}
})

func startOAuthServer() *ghttp.Server {
	server := ghttp.NewUnstartedServer()
	tlsConfig, err := cfhttp.NewTLSConfig("fixtures/server.crt", "fixtures/server.key", "")
	Expect(err).NotTo(HaveOccurred())
	tlsConfig.ClientAuth = tls.NoClientCert

	server.HTTPTestServer.TLS = tlsConfig
	server.AllowUnhandledRequests = true
	server.UnhandledRequestStatusCode = http.StatusOK

	server.HTTPTestServer.StartTLS()

	publicKey := "-----BEGIN PUBLIC KEY-----\\n" +
		"MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDHFr+KICms+tuT1OXJwhCUmR2d\\n" +
		"KVy7psa8xzElSyzqx7oJyfJ1JZyOzToj9T5SfTIq396agbHJWVfYphNahvZ/7uMX\\n" +
		"qHxf+ZH9BL1gk9Y6kCnbM5R60gfwjyW1/dQPjOzn9N394zd2FJoFHwdq9Qs0wBug\\n" +
		"spULZVNRxq7veq/fzwIDAQAB\\n" +
		"-----END PUBLIC KEY-----"

	data := fmt.Sprintf("{\"alg\":\"rsa\", \"value\":\"%s\"}", publicKey)
	server.RouteToHandler("GET", "/token_key",
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/token_key"),
			ghttp.RespondWith(http.StatusOK, data)),
	)
	server.RouteToHandler("POST", "/oauth/token",
		ghttp.CombineHandlers(
			ghttp.VerifyBasicAuth("someclient", "somesecret"),
			func(w http.ResponseWriter, req *http.Request) {
				jsonBytes := []byte(`{"access_token":"some-token", "expires_in":10}`)
				w.Write(jsonBytes)
			}))

	return server
}

var _ = BeforeEach(func() {
	cfgs = nil

	consulRunner.Start()
	consulRunner.WaitUntilReady()
	consulClusterAddress = consulRunner.ConsulCluster()

	sqlProcess = ginkgomon.Invoke(sqlRunner)

	startBBS()

	gnatsdRunner, natsClient = gnatsdrunner.StartGnatsd(natsPort)

	testMetricsChan = make(chan interface{}, 1)

	healthCheckPort = 4500 + GinkgoParallelNode()
	healthCheckAddress = fmt.Sprintf("127.0.0.1:%d", healthCheckPort)

	var err error
	dropsondePort, err = strconv.Atoi(strings.TrimPrefix(testMetricsListener.LocalAddr().String(), "127.0.0.1:"))
	Expect(err).NotTo(HaveOccurred())
})

var _ = JustBeforeEach(func() {
	if useLoggregatorV2 {
		var err error
		testIngressServer, err = testhelpers.NewTestIngressServer("fixtures/metron/metron.crt", "fixtures/metron/metron.key", "fixtures/metron/CA.crt")
		Expect(err).NotTo(HaveOccurred())
		metricsChan := testIngressServer.Receivers()
		testIngressServer.Start()
		port, err := strconv.Atoi(strings.TrimPrefix(testIngressServer.Addr(), "127.0.0.1:"))
		Expect(err).NotTo(HaveOccurred())
		cfgs = append(cfgs, func(cfg *config.RouteEmitterConfig) {
			cfg.LoggregatorConfig.UseV2API = true
			cfg.LoggregatorConfig.APIPort = port
			cfg.LoggregatorConfig.CACertPath = "fixtures/metron/CA.crt"
			cfg.LoggregatorConfig.KeyPath = "fixtures/metron/client.key"
			cfg.LoggregatorConfig.CertPath = "fixtures/metron/client.crt"
		})

		go func() {
			for {
				select {
				case data := <-metricsChan:
					batch, err := data.Recv()
					if err != nil {
						close(testMetricsChan)
						return
					}
					for _, elem := range batch.Batch {
						testMetricsChan <- elem
					}
				}
			}
		}()
	} else {
		testMetricsListener, _ = net.ListenPacket("udp", "127.0.0.1:0")
		go func() {
			defer GinkgoRecover()
			for {
				buffer := make([]byte, 1024)
				n, _, err := testMetricsListener.ReadFrom(buffer)
				if err != nil {
					close(testMetricsChan)
					return
				}

				var envelope events.Envelope
				err = proto.Unmarshal(buffer[:n], &envelope)
				Expect(err).NotTo(HaveOccurred())
				testMetricsChan <- &envelope
			}
		}()
	}
})

var _ = AfterEach(func() {
	stopBBS()
	consulRunner.Stop()
	gnatsdRunner.Signal(os.Kill)
	Eventually(gnatsdRunner.Wait(), 5).Should(Receive())

	if useLoggregatorV2 {
		testIngressServer.Stop()
	} else {
		testMetricsListener.Close()
	}
	Eventually(testMetricsChan).Should(BeClosed())

	ginkgomon.Kill(sqlProcess, 5*time.Second)
})

var _ = SynchronizedAfterSuite(func() {
	oauthServer.Close()
}, func() {
	gexec.CleanupBuildArtifacts()
})

func getServerPort(url string) string {
	endpoints := strings.Split(url, ":")
	Expect(endpoints).To(HaveLen(3))
	return endpoints[2]
}

func stopBBS() {
	if !bbsRunning {
		return
	}

	bbsRunning = false
	ginkgomon.Kill(bbsProcess)
	Eventually(bbsProcess.Wait()).Should(Receive())
}

func startBBS() {
	if bbsRunning {
		return
	}

	bbsRunner = bbstestrunner.New(bbsPath, bbsConfig)
	bbsProcess = ginkgomon.Invoke(bbsRunner)
	bbsRunning = true
}
