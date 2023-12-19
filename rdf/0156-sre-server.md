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
