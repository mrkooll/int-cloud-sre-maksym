package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// represents a Kubernetes deployment
type Deployment struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	ReplicaCount int32  `json:"replicaCount"`
}

// represents a error message
type ErrorMessage struct {
	Error string `json:"error"`
}

func main() {
	err := run(os.Args)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

// return in cluster or file config
func getConfig(inCluster bool, kubeconfig string) (*rest.Config, error) {
	if inCluster {
		return rest.InClusterConfig()
	} else {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
}

func run(args []string) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	var port, kubeconfig string
	var cert, key, cacert string
	flag.StringVar(&port, "port", "8080", "server port")
	inCluster := flag.Bool("incluster", false, "service is run inside cluster")
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(homedir, ".kube", "config"), "path to the kubeconfig file")
	flag.StringVar(&cert, "cert", "/etc/certs/sre-server.crt", "certificate file")
	flag.StringVar(&key, "key", "/etc/certs/sre-server.key", "certificate key file")
	flag.StringVar(&cacert, "cacert", "/etc/certs/sre-bundle.crt", "CA certificates file")
	flag.Parse()

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return err
	}

	// Load CA certificate
	caCert, err := os.ReadFile(cacert)
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Create TLS Config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}

	// use the current context in kubeconfig
	config, err := getConfig(*inCluster, kubeconfig)
	if err != nil {
		return err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Create a shared informer factory
	// TODO add defaultResync to the configuration
	sharedInformers := informers.NewSharedInformerFactory(clientset, time.Minute*5)

	// Create a deployment informer for all namespaces
	deploymentInformer := sharedInformers.Apps().V1().Deployments().Informer()

	// create a deployment lister for all namespaces
	deploymentLister := sharedInformers.Apps().V1().Deployments().Lister()

	// Start the informer factory to begin watching for changes
	// we run the informer for the lifetime of the application
	sharedInformers.Start(context.Background().Done())

	// Wait for the cache to sync before exiting
	if !cache.WaitForCacheSync(context.Background().Done(), deploymentInformer.HasSynced) {
		log.Panicln("Timed out waiting for caches to sync")
	}
	log.Println("Informer is started")
	// add router
	router := mux.NewRouter()
	// add handlers
	router.Handle("/api/v1/namespaces/{namespace}/deployments/{name}/replicas",
		&deploymentHandler{clientset: clientset, informer: deploymentInformer, lister: deploymentLister})
	router.Handle("/api/v1/deployments", &deploymentsHandler{lister: deploymentLister})
	router.Handle("/api/v1/healthz", &healthzHandler{clientset: clientset, informer: deploymentInformer})
	router.Handle("/api/v1/pingz", &pingzHandler{})

	// Create HTTP Server with TLS Config
	server := &http.Server{
		Addr:      ":" + port,
		TLSConfig: tlsConfig,
		Handler:   router,
	}

	// make channel for OS signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	// run https server in the goroutine
	go func() {
		log.Printf("Starting HTTPS server on port %s...", port)
		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %s", err)
		}
	}()
	// block execution until receive signal
	<-stopChan
	log.Println("Shutting down server...")

	// Initialize shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

// deploymentHandler is an HTTP handler for the deployment API.
type deploymentHandler struct {
	clientset *kubernetes.Clientset
	informer  cache.SharedIndexInformer
	lister    listersv1.DeploymentLister
}

func writeMessage(w http.ResponseWriter, statusCode int, data any) {
	d, err := json.Marshal(data)
	if err != nil {
		log.Panicf("Unable to marshal data %v", err)
	}
	w.WriteHeader(statusCode)
	_, err = w.Write(d)
	if err != nil {
		log.Printf("unable to write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, txt string) {
	d := ErrorMessage{Error: txt}
	writeMessage(w, statusCode, d)
}

// ServeHTTP implements http.Handler
func (h *deploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	// check is deployment exist
	deploymentSpec, err := h.lister.Deployments(namespace).Get(name)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("Deployment %s in namespace %s does not exist", name, namespace))
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Error while fetching deployment: %v", err))
		}
		return
	}

	switch r.Method {
	case "GET":
		// Handle GET request
		writeMessage(w, http.StatusOK, &Deployment{
			Namespace:    deploymentSpec.Namespace,
			Name:         deploymentSpec.Name,
			ReplicaCount: deploymentSpec.Status.Replicas})
		return

	case "PUT":
		// Handle PUT request
		// request body spec
		type ReplicaCountSpec struct {
			ReplicaCount int32 `json:"replicaCount"`
		}

		// get replica count
		// var spec ReplicaCountSpec
		spec := ReplicaCountSpec{ReplicaCount: -1}
		r.Body = http.MaxBytesReader(w, r.Body, 150) // limit request reading size
		defer r.Body.Close()
		err := json.NewDecoder(r.Body).Decode(&spec)

		if err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "Empty request body")
			} else if err == http.ErrHandlerTimeout {
				writeError(w, http.StatusRequestTimeout, "Request body too large")
			} else {
				writeError(w, http.StatusBadRequest, "Unable to decode request body: "+err.Error())
			}
			return
		}

		if spec.ReplicaCount < 0 {
			writeError(w, http.StatusBadRequest, "missing or invalid replicaCount")
			return
		}

		deploymentClient := h.clientset.AppsV1().Deployments(namespace)
		// creating patch for the deployment
		patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, spec.ReplicaCount))
		// apply patch
		_, err = deploymentClient.Patch(r.Context(), name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Error while patching deployment: "+err.Error())
		}
		writeMessage(w, http.StatusOK, &Deployment{
			Namespace:    deploymentSpec.Namespace,
			Name:         deploymentSpec.Name,
			ReplicaCount: spec.ReplicaCount})
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// deploymentsHandler is an HTTP handler for the deployments API.
type deploymentsHandler struct {
	lister listersv1.DeploymentLister
}

// ServeHTTP implements http.Handler
func (h *deploymentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// List all deployments in all namespaces
	deployments, err := h.lister.Deployments(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Error while fetching deployments: "+err.Error())
	} else {
		result := make([]*Deployment, 0, len(deployments))
		// Iterate over the deployments
		for _, deployment := range deployments {
			result = append(result, &Deployment{
				Namespace:    deployment.Namespace,
				Name:         deployment.Name,
				ReplicaCount: deployment.Status.Replicas,
			})
		}
		writeMessage(w, http.StatusOK, result)
	}
}

// healthzHandler is an HTTP handler for the healthz API.
type healthzHandler struct {
	clientset *kubernetes.Clientset
	informer  cache.SharedIndexInformer
}

// Return healthy status
// represents a status message
type HealthzStatus struct {
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Kubernetes string `json:"kubernetes,omitempty"`
}

// ServeHTTP implements http.Handler
func (h *healthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.informer.HasSynced() {
		_, err := h.clientset.AppsV1().Deployments("").List(r.Context(), metav1.ListOptions{Limit: 1})
		if err != nil {
			status := HealthzStatus{
				Status: "unhealthy",
				Error:  "unable to connect to the cluster: " + err.Error(),
			}
			writeMessage(w, http.StatusServiceUnavailable, status)
			return
		}
		status := HealthzStatus{Status: "healthy", Kubernetes: "connected"}
		writeMessage(w, http.StatusOK, status)
	} else {
		// Return unhealthy status
		status := HealthzStatus{
			Status: "unhealthy",
			Error:  "cluster informer is not synced yet"}
		writeMessage(w, http.StatusServiceUnavailable, status)
	}
}

type pingzHandler struct{}

func (h *pingzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type pingOk struct {
		Status string `json:"status"`
	}
	status := pingOk{Status: "alive"}
	writeMessage(w, http.StatusOK, status)
}
