package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/apps/v1"
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
		log.Printf("Error: %v", err)
	}
}

func run(args []string) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	var port, kubeconfig string
	flag.StringVar(&port, "port", "8080", "server port")
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(homedir, ".kube", "config"), "path to the kubeconfig file")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
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

	// Add an event handler to print deployment details when they change
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// deployment := obj.(*metav1.PartialObjectMetadata)
			deployment, ok := obj.(*appsv1.Deployment)
			if !ok {
				// Handle the error appropriately
				log.Println("object is not a deployment")
				return
			}
			log.Printf("deployment added: %s/%s (replicas: %d)\n",
				deployment.Namespace,
				deployment.Name,
				deployment.Status.Replicas)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// deployment := newObj.(*metav1.PartialObjectMetadata)
			deployment, ok := newObj.(*appsv1.Deployment)
			if !ok {
				// Handle the error appropriately
				log.Println("Object is not a Deployment")
				return
			}
			log.Printf("Deployment updated: %s/%s (replicas: %d)\n",
				deployment.Namespace,
				deployment.Name,
				deployment.Status.Replicas)
		},
		DeleteFunc: func(obj interface{}) {
			// deployment := obj.(*metav1.PartialObjectMetadata)
			deployment, ok := obj.(*appsv1.Deployment)
			if !ok {
				// Handle the error appropriately
				log.Println("Object is not a Deployment")
				return
			}
			log.Printf("Deployment deleted: %s/%s\n",
				deployment.Namespace,
				deployment.Name)
		},
	})

	// Start the informer factory to begin watching for changes
	// we run the informer for the lifetime of the application
	sharedInformers.Start(context.Background().Done())

	// Wait for the cache to sync before exiting
	if !cache.WaitForCacheSync(context.Background().Done(), deploymentInformer.HasSynced) {
		log.Panicln("Timed out waiting for caches to sync")
	}
	// add router
	router := mux.NewRouter()
	// add handlers
	router.Handle("/v1/namespaces/{namespace}/deployments/{name}/replicas",
		&deploymentHandler{clientset: clientset, lister: deploymentLister})
	router.Handle("/v1/deployments", &deploymentsHandler{lister: deploymentLister})
	router.Handle("/v1/healthz", &healthzHandler{clientset: clientset, informer: deploymentInformer})
	router.Handle("/v1/pingz", &pingzHandler{})

	return http.ListenAndServe(":"+port, router)
}

// deploymentHandler is an HTTP handler for the deployment API.
type deploymentHandler struct {
	clientset *kubernetes.Clientset
	lister    listersv1.DeploymentLister
}

func writeMessage(w http.ResponseWriter, statusCode int, data any) {
	d, _ := json.Marshal(data)
	w.WriteHeader(statusCode)
	_, err := w.Write(d)
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
		err := json.NewDecoder(r.Body).Decode(&spec)
		defer r.Body.Close()

		if err != nil {
			// bad data type in json or data that doesn't math the spec
			errTxt := "unable to decode request body"
			if err != nil {
				errTxt += errTxt + " " + err.Error()
			}
			writeError(w, http.StatusBadRequest, errTxt)
			return
		}

		if spec.ReplicaCount < 0 {
			writeError(w, http.StatusBadRequest, "missing or invalid replicaCount")
			return
		}

		// fetch the deployment
		deploymentClient := h.clientset.AppsV1().Deployments(namespace)

		deployment, err := deploymentClient.Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "error while fetching deployment: "+err.Error())
			return
		}
		// set replica count
		deployment.Spec.Replicas = &spec.ReplicaCount
		// update the deployment
		_, err = deploymentClient.Update(r.Context(), deployment, metav1.UpdateOptions{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Error while updating deployment: "+err.Error())
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
// represents a healthy message
type HealthyStatus struct {
	Status     string `json:"status"`
	Kubernetes string `json:"kubernetes"`
}

// represents an unhealthy message
type UnhealthyStatus struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// ServeHTTP implements http.Handler
func (h *healthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.informer.HasSynced() {
		_, err := h.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{Limit: 1})
		if err != nil {
			status := UnhealthyStatus{
				Status: "unhealthy",
				Error:  "unable to connect to the cluster: " + err.Error(),
			}
			writeMessage(w, http.StatusServiceUnavailable, status)
			return
		}
		status := HealthyStatus{Status: "healthy", Kubernetes: "connected"}
		writeMessage(w, http.StatusOK, status)
	} else {
		// Return unhealthy status
		status := UnhealthyStatus{
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
