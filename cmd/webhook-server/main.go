package main

import (
	"admission-webhook-oversale-sample/cmd/config"
	"fmt"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
	"path/filepath"
)

const (
	tlsDir         = `/run/secrets/tls`
	tlsCertFile    = `cert.pem`
	tlsKeyFile     = `key.pem`
	toolAnnotation = `webhook.citiccard.com`
)

var (
	nodeResource = metav1.GroupVersionResource{Version: "v1", Resource: "nodes"}
)

func getPatchItem(op string, path string, val interface{}) patchOperation {
	return patchOperation{
		Op:    op,
		Path:  path,
		Value: val,
	}
}

func initPatch(node corev1.Node) []patchOperation {
	var patches []patchOperation
	patches = append(patches, getPatchItem("replace", "/status/allocatable/cpu", "12"))
	return patches
}

func applyNodeConfig(req *v1beta1.AdmissionRequest, toolConfig *config.ToolConfig) ([]patchOperation, error) {
	if req.Resource != nodeResource {
		log.Printf("expect resource to be %s", nodeResource)
		return nil, nil
	}
	// Parse the Node object.
	raw := req.Object.Raw
	node := corev1.Node{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &node); err != nil {
		return nil, fmt.Errorf("could not deserialize node object: %v", err)
	}
	var patches []patchOperation
	patches = initPatch(node)
	return patches, nil
}
func main() {
	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)
	toolConfig := config.NewToolConfig()
	mux := http.NewServeMux()
	log.Printf("listen on port 8443")
	mux.Handle("/mutate", admitFuncHandler(applyNodeConfig, &toolConfig))
	server := &http.Server{
		// We listen on port 8443 such that we do not need root privileges or extra capabilities for this server.
		// The Service object will take care of mapping this port to the HTTPS port 443.
		Addr:    ":8443",
		Handler: mux,
	}
	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}
