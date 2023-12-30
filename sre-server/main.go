package main

import (
	"context"
	//	"crypto/tls"
	"encoding/json"
	"flag"

	// "fmt"
	//"errors"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	// "k8s.io/apimachinery/pkg/labels"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
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
				log.Println("Object is not a Deployment")
				return
			}
			log.Printf("Deployment added: %s/%s (replicas: %d)\n", deployment.Namespace, deployment.Name, deployment.Status.Replicas)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// deployment := newObj.(*metav1.PartialObjectMetadata)
			deployment, ok := newObj.(*appsv1.Deployment)
			if !ok {
				// Handle the error appropriately
				log.Println("Object is not a Deployment")
				return
			}
			log.Printf("Deployment updated: %s/%s (replicas: %d)\n", deployment.Namespace, deployment.Name, deployment.Status.Replicas)
		},
		DeleteFunc: func(obj interface{}) {
			// deployment := obj.(*metav1.PartialObjectMetadata)
			deployment, ok := obj.(*appsv1.Deployment)
			if !ok {
				// Handle the error appropriately
				log.Println("Object is not a Deployment")
				return
			}
			log.Printf("Deployment deleted: %s/%s\n", deployment.Namespace, deployment.Name)
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
	router.Handle("/v1/namespaces/{namespace}/deployments/{name}/replicas", &deploymentHandler{clientset: clientset, lister: deploymentLister})
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

// ServeHTTP implements http.Handler
func (h *deploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]
	result := Deployment{}
	errorResult := ErrorMessage{}
	hasError := false

	// check is deployment exist
	deployment, err := h.lister.Deployments(namespace).Get(name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			errorResult.Error = "Deployment " + name + " in namespace " + namespace + "does not exist"
			w.WriteHeader(http.StatusNotFound)

		} else {
			errorResult.Error = "Error while fetching deployment:" + err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		}
		hasError = true
	}

	switch r.Method {
	case "GET":
		// Handle GET request
		if !hasError {
			result.Name = name
			result.Namespace = namespace
			result.ReplicaCount = *deployment.Spec.Replicas
		}
	case "PUT":
		// Handle PUT request
		// request body spec
		type ReplicaCountSpec struct {
			ReplicaCount int `json:"replicaCount"`
		}
		if !hasError {
			// get replica count
			// var spec ReplicaCountSpec
			spec := ReplicaCountSpec{ReplicaCount: -1}
			err := json.NewDecoder(r.Body).Decode(&spec)
			defer r.Body.Close()
			if err != nil || spec.ReplicaCount == -1 {
				// bad data type in json or data that doesn't math the spec
				hasError = true
				if err != nil {
					errorResult.Error = "Unable to decode request body " + err.Error()
				} else {
					errorResult.Error = "Unable to decode request body"
				}
				w.WriteHeader(http.StatusBadRequest)
			} else {
				result.Name = name
				result.Namespace = namespace
				result.ReplicaCount = int32(spec.ReplicaCount)

				// fetch the deployment
				deploymentClient := h.clientset.AppsV1().Deployments(result.Namespace)
				deployment, err := deploymentClient.Get(context.TODO(), result.Name, metav1.GetOptions{})
				if err != nil {
					hasError = true
					errorResult.Error = "Error while fetching deployment:" + err.Error()
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					// set replica count
					replicaCount := result.ReplicaCount
					deployment.Spec.Replicas = &replicaCount
					// update the deployment
					_, err = deploymentClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
					if err != nil {
						hasError = true
						errorResult.Error = "Error while updating deployment:" + err.Error()
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
			}
		}
	default:
		errorResult.Error = "Method not allowed"
		w.WriteHeader(http.StatusMethodNotAllowed)
		hasError = true
	}
	if hasError {
		jsonData, er := json.Marshal(errorResult)
		if er != nil {
			log.Fatalf("Error occurred during marshaling. Error: %s", er.Error())
		}
		w.Write([]byte(jsonData))
	} else {
		// everything ok
		w.WriteHeader(http.StatusOK)
		jsonData, er := json.Marshal(result)
		if er != nil {
			log.Fatalf("Error occurred during marshaling. Error: %s", er.Error())
		}
		w.Write([]byte(jsonData))
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
		errorMsg := ErrorMessage{Error: err.Error()}
		jsonData, er := json.Marshal(errorMsg)
		if er != nil {
			log.Fatalf("Error occurred during marshaling. Error: %s", er.Error())
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(jsonData))
	} else {
		result := []Deployment{}
		// Iterate over the deployments
		for _, deployment := range deployments {
			result = append(result, Deployment{Namespace: deployment.Namespace, Name: deployment.Name, ReplicaCount: deployment.Status.Replicas})
		}
		jsonData, er := json.Marshal(result)
		if er != nil {
			log.Fatalf("Error occurred during marshaling. Error: %s", er.Error())
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonData))
	}
}

// healthzHandler is an HTTP handler for the healthz API.
type healthzHandler struct {
	clientset *kubernetes.Clientset
	informer  cache.SharedIndexInformer
}

// ServeHTTP implements http.Handler
func (h *healthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	healthy := true
	errorMsg := ""
	if h.informer.HasSynced() {
		_, err := h.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{Limit: 1})
		if err == nil {
			// Return healthy status
			// represents a healthy message
			type HealthyMessage struct {
				Status     string `json:"status"`
				Kubernetes string `json:"kubernetes"`
			}
			status := HealthyMessage{Status: "healthy", Kubernetes: "connected"}
			jsonData, err := json.Marshal(status)
			if err != nil {
				log.Fatalf("Error occurred during marshaling. Error: %s", err.Error())
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(jsonData))
		} else {
			healthy = false
			errorMsg = "Unable to connect to the cluster: " + err.Error()
		}
	} else {
		healthy = false
		errorMsg = "Informer lost synchronization with the cluster"
	}
	if !healthy {
		// Return error status
		// represents a unhealthy message
		type UnHealthyMessage struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		status := UnHealthyMessage{Status: "unhealthy", Error: errorMsg}
		jsonData, err := json.Marshal(status)
		if err != nil {
			log.Fatalf("Error occurred during marshaling. Error: %s", err.Error())
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(jsonData))
	}
}

type pingzHandler struct{}

func (h *pingzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type pingOk struct {
		Status string `json:"status"`
	}
	status := pingOk{Status: "alive"}
	jsonData, err := json.Marshal(status)
	if err != nil {
		log.Fatalf("Error occurred during marshaling. Error: %s", err.Error())
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(jsonData))
}
