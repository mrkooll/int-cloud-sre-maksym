---
authors: Maksym Tiurin (maksym@tiurin.name)
state: draft
---

# RFD 156 - Enhancing Kubernetes Interaction in Go Server for SRE Level 4

## What

This RFD proposes enhancements to a Go server for efficient
interaction with Kubernetes resources, specifically targeting Level 4
requirements in the SRE challenge. The key enhancement is the
integration of Kubernetes Informers for efficient resource monitoring
and management, coupled with securing connections through mTLS.

## Why

Managing Kubernetes resources efficiently is essential in modern
cloud-native environments. Kubernetes Informers provide a more
efficient way to watch for resource changes compared to direct Watch
API usage, by maintaining a local store and reducing direct API
calls. Coupling this with secure communication via mTLS ensures both
performance and security are addressed.

## Details

### Server

* **HTTP API with Kubernetes Informers**: Implement Kubernetes
  Informers to monitor and cache changes in Deployment
  resources. Informers provide an event-based mechanism to react to
  changes in Kubernetes objects, thus reducing the need for frequent
  API calls and improving performance.
* **mTLS for Secure Communication**: Establish mutual TLS (mTLS) for
  secure and authenticated communication between the server and
  clients.

### API Specification

#### Base URL

The base URL for the API will be:
```
https://<server-address>:<port>/api/v1
```

**Note**: The actual server address and port will be provided in the
service deployment documentation.

#### Authentication

All API requests must be authenticated using mTLS. Clients must
present a valid client certificate that the server will authenticate
against a known CA.

#### Endpoints

1. Get Replica Count of a Kubernetes Deployment
    * **Endpoint**: `/deployments/{name}/replicas`
	* **Method**: `GET`
	* **URL Params**:
		- `name`: Name of the Kubernetes Deployment
	* **Response**:
		- Success: HTTP 200
			- `Content: { "name": "<deployment_name>", "replicaCount": <count> }`
		- Error: HTTP 4xx/5xx (appropriate error status code)
			- `Content: { "error": "<error_message>" }`
2. Set Replica Count of a Kubernetes Deployment
    * **Endpoint**: `/deployments/{name}/replicas`
	* **Method**: `PUT`
	* **URL Params**:
		- `name`: Name of the Kubernetes Deployment
	* **Request Body**:
		- `Content: { "replicaCount": <new_count> }`
	* **Response**:
		- Success: HTTP 200
			- `Content: { "message": "Replica count updated successfully." }`
		- Error: HTTP 4xx/5xx (appropriate error status code)
			- `Content: { "error": "<error_message>" }`
3. List Available Kubernetes Deployments
    * **Endpoint**: `/deployments`
	* **Method**: `GET`
	* **Response**:
		- Success: HTTP 200
			- `Content: [{ "name": "<deployment_name>", "replicaCount": <count> }, ...]`
		- Error: HTTP 4xx/5xx (appropriate error status code)
			- `Content: { "error": "<error_message>" }`
4. Health Check
    * **Endpoint**: `/health`
	* **Method**: `GET`
	* **Response**:
		- Success: HTTP 200
			- `Content: { "status": "healthy", "kubernetes": "connected" }`
		- Error: HTTP 4xx/5xx (appropriate error status code)
			- `Content: { "status": "unhealthy", "error": "<error_message>" }`

#### Error Handling

All endpoints should return meaningful HTTP status codes and error
messages in case of failures, including but not limited to invalid
requests, authentication errors, and internal server errors.

#### Security and Data Integrity

* All data exchanged with the API is encrypted using TLS.
* mTLS is used for client authentication to ensure that only
  authorized clients can access the API.
* Input validation is performed on all incoming requests to prevent
  common web vulnerabilities.

### Automation

* **Make-based Workflow**: Use `make` for building, testing, and
  deploying the server, ensuring consistent and automated processes.

### Security

* **mTLS Setup**: Detail the mTLS implementation, focusing on
  certificate management and secure cipher suites.
* **Informer Security Considerations**: Analyze security aspects
  specific to Informers, such as access controls and data integrity. 

### UX

* **API Usage**: Describe the interaction with new HTTP API endpoints,
  showcasing request/response formats and error handling. 
* **Configuration Management**: Discuss how these changes impact
  existing setups and the upgrade path.

### Proto Specification

* Not applicable at this level (relevant for Level 5 with gRPC API).

### Backward Compatibility

* **Impact Assessment**: Evaluate how new features affect existing
  clients and provide necessary migration steps.

### Audit Events

* **Logging Strategies**: Define new logging requirements for
  monitoring the usage of Kubernetes Informers and mTLS.

### Observability

* **Performance Monitoring**: Introduce Prometheus metrics to track
  the performance improvements offered by Informers.
* **Error Logging**: Implement comprehensive logging for error
  detection and resolution.

### Product Usage

* **Telemetry Updates**: Propose updates to telemetry to capture the
  adoption and usage of the new features.

### Test Plan

* **Testing Strategy**: Outline a comprehensive test plan for the
  integration of Kubernetes Informers and mTLS, including both
  positive and negative scenarios.
