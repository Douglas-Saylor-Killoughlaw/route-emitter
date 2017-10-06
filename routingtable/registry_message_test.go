package routingtable_test

import (
	"encoding/json"

	"code.cloudfoundry.org/route-emitter/routingtable"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistryMessage", func() {
	var expectedMessage routingtable.RegistryMessage

	BeforeEach(func() {
		expectedMessage = routingtable.RegistryMessage{
			Host:                 "1.1.1.1",
			Port:                 61001,
			URIs:                 []string{"host-1.example.com"},
			App:                  "app-guid",
			PrivateInstanceId:    "instance-guid",
			PrivateInstanceIndex: "0",
			RouteServiceUrl:      "https://hello.com",
			Tags:                 map[string]string{"component": "route-emitter"},
		}
	})

	Describe("serialization", func() {
		var expectedJSON string

		BeforeEach(func() {
			expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"private_instance_index": "0",
				"route_service_url": "https://hello.com",
				"tags": {"component":"route-emitter"}
			}`
		})

		It("marshals correctly", func() {
			payload, err := json.Marshal(expectedMessage)
			Expect(err).NotTo(HaveOccurred())

			Expect(payload).To(MatchJSON(expectedJSON))
		})

		It("unmarshals correctly", func() {
			message := routingtable.RegistryMessage{}

			err := json.Unmarshal([]byte(expectedJSON), &message)
			Expect(err).NotTo(HaveOccurred())
			Expect(message).To(Equal(expectedMessage))
		})

		Context("when TLS port is set", func() {
			BeforeEach(func() {
				expectedMessage.TlsPort = 61007

				expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"tls_port": 61007,
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"private_instance_index": "0",
				"route_service_url": "https://hello.com",
				"tags": {"component":"route-emitter"}
			}`
			})

			It("correctly marshals the TLS port", func() {
				message := routingtable.RegistryMessage{}

				err := json.Unmarshal([]byte(expectedJSON), &message)
				Expect(err).NotTo(HaveOccurred())
				Expect(message).To(Equal(expectedMessage))
			})
		})
	})

	Describe("RegistryMessageFor", func() {
		var endpoint routingtable.Endpoint
		var route routingtable.Route

		BeforeEach(func() {
			endpoint = routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         0,
				Host:          "1.1.1.1",
				Port:          61001,
				ContainerPort: 11,
			}

			route = routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
			}
		})

		It("creates a valid message from an endpoint and routes", func() {
			message := routingtable.RegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})

		It("sets the TLS port if a TLS proxy port is provided", func() {
			expectedMessage.TlsPort = 61005
			endpoint.TlsProxyPort = 61005

			message := routingtable.RegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message when instance index is greater than 0", func() {
			expectedMessage.PrivateInstanceIndex = "2"
			endpoint.Index = 2

			message := routingtable.RegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})
	})

	Describe("InternalAddressRegistryMessageFor", func() {
		var endpoint routingtable.Endpoint
		var route routingtable.Route

		BeforeEach(func() {
			expectedMessage = routingtable.RegistryMessage{
				Host:                 "1.2.3.4",
				Port:                 11,
				URIs:                 []string{"host-1.example.com"},
				App:                  "app-guid",
				PrivateInstanceId:    "instance-guid",
				PrivateInstanceIndex: "0",
				RouteServiceUrl:      "https://hello.com",
				Tags:                 map[string]string{"component": "route-emitter"},
			}

			endpoint = routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         0,
				Host:          "1.1.1.1",
				ContainerIP:   "1.2.3.4",
				Port:          61001,
				ContainerPort: 11,
			}
			route = routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
			}

		})

		It("creates a valid message from an endpoint and routes", func() {
			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})

		It("sets the TLS port in the message if the container TLS proxy port is set", func() {
			expectedMessage.TlsPort = 61007
			endpoint.ContainerTlsProxyPort = 61007

			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message when instance index is greater than 0", func() {
			expectedMessage.PrivateInstanceIndex = "2"
			endpoint.Index = 2

			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})
	})

	Describe("InternalEndpointRegistryMessageFor", func() {
		var endpoint routingtable.Endpoint
		var route routingtable.InternalRoute

		BeforeEach(func() {
			expectedMessage = routingtable.RegistryMessage{
				Host:                 "1.2.3.4",
				URIs:                 []string{"host-1.example.com", "0.host-1.example.com"},
				App:                  "app-guid",
				Tags:                 map[string]string{"component": "route-emitter"},
				PrivateInstanceIndex: "0",
			}

			endpoint = routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         0,
				Host:          "1.1.1.1",
				ContainerIP:   "1.2.3.4",
				Port:          61001,
				ContainerPort: 11,
			}

			route = routingtable.InternalRoute{
				Hostname:    "host-1.example.com",
				LogGUID:     "app-guid",
				ContainerIP: "5.6.7.8",
			}
		})

		It("creates a valid message from an endpoint and routes", func() {
			message := routingtable.InternalEndpointRegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})
	})
})
