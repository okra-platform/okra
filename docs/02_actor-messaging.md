# 02 – Actor Messaging and Service Routing

## Service Invocation via Actor Messaging

In OKRA, calling a service means sending a message to a GoAKT actor that has been registered under the service's fully qualified Protobuf service name.

Example:
- Service: `okra.user.v1.UserService`
- Actor ID: `"okra.user.v1.UserService"` (used as the address for message routing)

All service invocations happen by passing messages to this actor.

---

## Message Envelope: `ServiceRequest`

All services are invoked through a generic Protobuf envelope:

```protobuf
message ServiceRequest {
  string method = 1;          // Fully qualified method name (e.g. GetUser)
  bytes input = 2;            // Raw serialized Protobuf message
  map<string, string> metadata = 3; // Optional context (e.g. headers, auth)
}
```

## Service Registry

The OKRA Service Registry is responsible for mapping service names to their deployed WASM packages and type information. It enables dynamic service discovery, validation, and routing.

Things to add to registry:
- Service name & metadata/config (okra.service.json)
- Pointer to .pkg in s3
- Pointer to extracted bits
  - Extracted okra.service.json
  - Extracted `.wasm`
  - Extracted protobuf descriptopns `.pb.bin` 

The schema registry can expose service info to the cluster and the location to download the .wasm or .pb.bin


