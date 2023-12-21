---
authors: Maksym Tiurin (maksym@tiurin.name)
state: draft
---

# RFD 156 - Replica Count Management Server

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

### Automation

* **Make-based Workflow**: Use `make` for building, testing, and
  deploying the server, ensuring consistent and automated processes.

### Security

#### mTLS Setup

The current implementation of mutual TLS (mTLS) focuses on
foundational security checks to ensure secure 
communication. Specifically, the server performs two key validations
for mTLS:
  
* **Validation of Client Certificate Validity**: The server
  verifies that the client's SSL/TLS certificate is valid. This
  check ensures that the certificate has not expired and its
  data integrity is intact, confirming that the client's
  identity as presented in the certificate is valid at the time
  of the connection.
* **Verification of Certificate Issuance by a Known CA**: The
  server checks if the client's certificate is issued by a
  Certificate Authority (CA) that is known and trusted. This
  step involves comparing the certificate's issuer against a
  list of trusted CAs configured in the server. This ensures
  that the server only establishes secure connections with
  clients bearing certificates from recognized and trusted
  entities.
* **Use of Self-Signed CA**: In this initial version, we are utilizing
  a self-signed CA for simplicity and ease of setup. This approach is
  adequate for internal, development, or testing
  environments. However, for production environments, it is
  recommended to use certificates issued by a trusted CA.
* **Enforcing TLS 1.3**: To ensure that we do not use insecure or
  outdated protocols, the server is configured to only support TLS
  1.3, the latest version of the TLS protocol. This is achieved by
  setting the minimum TLS version to TLS 1.3 in the server's TLS
  configuration. This ensures enhanced security and performance
  benefits provided by TLS 1.3, such as improved encryption methods
  and streamlined handshake processes.
		
These checks form the core of the mTLS implementation in its
initial version, providing a secure foundation for
client-server communication. Future enhancements may include
more sophisticated certificate checks, certificate management
and secure cipher suites.

##### Planned Enhancements for the Next Version

* **Integration with Kubernetes Certificate Manager**: In the next
  version, it is planned to integrate with Kubernetes Certificate
  Manager for the management of TLS certificates. This integration
  will automate the process of issuing, renewing, and revoking
  certificates. It will leverage Kubernetes' native capabilities for
  certificate management, providing a more robust and scalable
  solution.
* **Transition to Kubernetes-Managed Certificates**: By using
  Kubernetes Certificate Manager, we aim to streamline certificate
  management in a cloud-native environment. This will allow us to
  efficiently manage the lifecycle of certificates and ensure
  compliance with security policies.

These checks and configurations form the core of the mTLS
implementation in its initial version, providing a secure foundation
for client-server communication. The planned enhancements aim to
further strengthen the security and scalability of our mTLS setup,
aligning with best practices in cloud-native environments.

#### Informer Security Considerations

In the initial version of our implementation, the security setup for
Kubernetes Informers is kept basic:

1. **Unrestricted Access**: The Informer operates without specific access
   restrictions. It uses a ServiceAccount that is not bound by any
   Role or RoleBinding, allowing it to access a wide range of
   resources within the Kubernetes cluster.
2. **Basic Service Account Setup**: A standard ServiceAccount is created
   and utilized by the Informer. This account does not yet include
   advanced security configurations but provides the necessary
   privileges for the Informer to function effectively.

This approach allows for initial testing and setup without the
complexity of detailed access controls. However, it's important to
note that this unrestricted access poses potential security risks, as
the Informer has broad permissions across the cluster.

##### Planned Future Enhancements

In the next version, we plan to significantly enhance the security
posture of the Informer with the following additions:

1. **Role-Based Access Control (RBAC)**: We will introduce RBAC to
   precisely define and limit the permissions of the ServiceAccount
   used by the Informer. This includes creating specific Roles and
   RoleBindings that grant only the necessary privileges required for
   the Informer's operations.
2. **Dedicated Namespace**: The Informer will operate within its own
   Namespace, isolating its activities and reducing the potential
   impact on other cluster resources.
3. **Secrets Management**: We will implement more robust handling of
   sensitive data, including managing Kubernetes Secrets. This ensures
   that sensitive information is accessed and utilized securely by the
   Informer, with proper controls to prevent unauthorized access or
   exposure.

By progressively enhancing the security framework around the Informer,
we aim to strike a balance between functional testing in the initial
phase and robust, secure operations in the subsequent version. This
phased approach allows us to ensure that the Informer operates
effectively while adhering to best practices in Kubernetes security.

### UX

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
    * **Endpoint**: `/namespaces/{namespace}/deployments/{name}/replicas`
	* **Method**: `GET`
	* **URL Params**:
		- `namespace`: Namespace of the Kubernetes Deployment
		- `name`: Name of the Kubernetes Deployment
	* **Response**:
		- Success: HTTP 200
			- `Content: { "namespace": "<namespace_name>", "name": "<deployment_name>", "replicaCount": <count> }`
		- Error: HTTP 4xx/5xx (appropriate error status code)
			- `Content: { "error": "<error_message>" }`
2. Set Replica Count of a Kubernetes Deployment
    * **Endpoint**: `/namespaces/{namespace}/deployments/{name}/replicas`
	* **Method**: `PUT`
	* **URL Params**:
		- `namespace`: Namespace of the Kubernetes Deployment
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
			- `Content: [{ "namespace": "<namespace>", "name": "<deployment_name>", "replicaCount": <count> }, ...]`
		- Error: HTTP 4xx/5xx (appropriate error status code)
			- `Content: { "error": "<error_message>" }`
4. Service Health Check
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

#### Planned Future Enhancements

In the next version of our application, we plan to evolve our API
by:

* Add rate limiting and throttling mechanisms to protest service from
  abuse.
* Add more methods to update deployment parameters like updating image
  version.

* **Configuration Management**: Discuss how these changes impact
  existing setups and the upgrade path.


### Proto Specification

* Not applicable for REST API.

### Backward Compatibility

* Not applicable for a new feature.

### Audit Events

#### Logging Strategies

##### Current Initial Version

In the initial version of our implementation, we are employing a
straightforward and traditional approach to logging:

* **Syslog-Style Text Logging to STDOUT/STDERR**: The application will
  utilize a simple, syslog-style text logging system. All logs
  generated by the application, including those related to the
  activities of Kubernetes Informers and the mTLS process, will be
  output in a textual format directly to STDOUT/STDERR.
* **Log Content**: These logs will include timestamped entries
  detailing operational events, errors, and warnings. For Kubernetes
  Informers, this includes events like resource changes, cache
  updates, and connection issues. For mTLS, log entries will cover
  certificate validations, authentication successes or failures, and
  any relevant security exceptions.
* **Simplicity and Accessibility**: This method of logging is chosen
  for its simplicity and ease of integration. It allows for quick
  access to logs and straightforward troubleshooting in the early
  stages of development.

##### Planned Future Enhancements

In the next version of our application, we plan to evolve our logging
strategy to align with more advanced practices:

* **Structured Logging**: We will implement structured logging to
  provide more context-rich and easily parsable logs. Structured logs
  are typically formatted in JSON, making them more suitable for
  complex applications and easier to integrate with modern log
  analysis tools.
* **Logging Details**: The structured logs will include detailed
  information such as event types, severity levels, contextual data,
  and unique identifiers for tracing. This is particularly beneficial
  for in-depth monitoring and analysis of Kubernetes Informer
  activities and mTLS processes.
* **Sidecar Container for Logging**: To further enhance our logging
  capabilities, we may introduce a sidecar container dedicated to log
  processing. This container can handle tasks like log aggregation,
  filtering, and forwarding to external monitoring systems or log
  databases.
* **Improved Scalability and Maintenance**: Structured logging,
  especially when combined with a sidecar container, allows for more
  scalable log management and easier maintenance. It provides a
  foundation for more sophisticated monitoring and alerting systems as
  the application grows in complexity.

By starting with a basic logging approach and planning for a
transition to structured logging, we aim to balance immediate
operational needs with a path towards more robust, scalable, and
maintainable logging practices in the future.

### Test Plan

#### Testing Strategy

For the initial version of our application, the testing strategy is
focused on basic functional testing to ensure core functionalities of
Kubernetes Informers and mTLS are working as expected.

* **Bash Script with Curl for Basic Functional Tests**:
	- We will use a simple Bash script that employs curl commands to
      interact with the application's endpoints.
    - This script will test the primary functionalities such as
      retrieving and setting the replica count of Kubernetes
      Deployments (via Informers) and ensuring mTLS-based secure
      communication.
    - **Positive Scenarios**: The script will include tests for
      expected behaviors, like successful retrieval of deployment data
      and successful updates of replica counts.
    - **Negative Scenarios**: The script will also handle negative
      test cases, such as attempting to access the API without a valid
      client certificate (to test mTLS) and requesting data on
      non-existent deployments.
* **Manual Review of Logs**:
	- The output logs of the application will be manually reviewed to
      ensure that Informer activities and mTLS processes are logged as
      expected. This includes checking for proper logging of both
      successful operations and any errors or warnings.

#### Planned Enhancements for the Next Version

As we progress to the next version, our testing strategy will become
more sophisticated with the integration of unit testing and automated
test frameworks.

* Integration of Gocheck Framework for Golang Unit Tests:
	- We will implement unit tests using the Gocheck framework, which
      provides more powerful testing capabilities for Go applications.
    - These tests will cover a broader range of scenarios and include
      more detailed assertions to validate the internal workings of
      our application.
    - Unit Testing Kubernetes Informers: Tests will be designed to
      simulate various Kubernetes events and assess the Informer's
      response to these events, ensuring proper cache updates and
      error handling.
    - Unit Testing mTLS Implementations: We will create tests to
      simulate different TLS scenarios, including certificate
      validation, client authentication, and error handling in case of
      certificate issues.
* Automated Test Execution:
	- The Gocheck tests will be integrated into our CI/CD pipeline,
      allowing for automated execution of tests upon each code commit
      or pull request.
	- This automation ensures consistent testing and helps in
      identifying issues early in the development cycle.

By starting with basic functional testing using a Bash script and
evolving towards comprehensive unit testing with Gocheck, this
strategy ensures that our application's critical features related to
Kubernetes Informers and mTLS are thoroughly validated at each stage
of development.
