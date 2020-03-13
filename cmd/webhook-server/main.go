/*Copyright (c) 2019 StackRox Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"toolcase.demo.com/cmd/config"
	"fmt"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

const (
	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `cert.pem`
	tlsKeyFile  = `key.pem`
	toolAnnotation = `webhook.citiccard.com`

)

var (
	podResource = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
)

func getPatchItem(op string, path string, val interface{}) patchOperation {
	return patchOperation {
		Op:    op,
		Path:  path,
		Value: val,
	}
}
func constructCmd(toolConfig *config.ToolConfig, toolKeyList []string) []string {
	var cmd []string
	var flag = false
	for _, toolName := range toolKeyList {
		if tool := toolConfig.GetTool(toolName); &tool != nil {
			if flag {
				cmd = append(cmd, "&&")
			}
			cmd = append(cmd, "cp", "-r", tool.Path, "/tools")
			flag = true
		}
	}
	return cmd
}

func initPatch(toolConfig *config.ToolConfig, toolKeyList []string, pod corev1.Pod) []patchOperation {
	var patches []patchOperation
	initContainers := pod.Spec.InitContainers
	volumeMount := corev1.VolumeMount{
		Name:             "tool-volume",
		ReadOnly:         true,
		MountPath:        "/tools",
	}
	tmpContainer := corev1.Container{
		Name:                     "tool",
		Image:                    toolConfig.Image,
		Command:                  constructCmd(toolConfig, toolKeyList),
		Resources:                corev1.ResourceRequirements{},
		VolumeMounts:             []corev1.VolumeMount{volumeMount},
		ImagePullPolicy:          "Always",
	}
	volume := corev1.Volume{
		Name:         "tool-volume",
		VolumeSource: corev1.VolumeSource{
			EmptyDir:              &corev1.EmptyDirVolumeSource{},
		},
	}


	if initContainers != nil {
		patches = append(patches, getPatchItem("add", "/spec/initContainers/-", tmpContainer))
	} else {
		patches = append(patches, getPatchItem("add", "/spec/initContainers", []corev1.Container{tmpContainer}))
	}
	patches = append(patches, getPatchItem("add", "/spec/containers/0/volumeMounts/-", volumeMount))
	patches = append(patches, getPatchItem("add", "/spec/volumes/-", volume))
	return patches

}

func applyToolConfig(req *v1beta1.AdmissionRequest, toolConfig *config.ToolConfig) ([]patchOperation, error)  {

	if req.Resource != podResource {
		log.Printf("expect resource to be %s", podResource)
		return nil, nil
	}

	// Parse the Pod object.
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return nil, fmt.Errorf("could not deserialize pod object: %v", err)
	}
	if pod.Annotations == nil {
		return nil, nil
	}

	var toolKeyList []string
	for key, val := range pod.Annotations {
		if strings.HasPrefix(key, toolAnnotation) && val == "true" {
			toolKeyList = append(toolKeyList, strings.Split(key, "/")[1])
		}
	}
	var patches []patchOperation
	if len(toolKeyList) > 0 {
		patches = initPatch(toolConfig, toolKeyList, pod)
		pod.Annotations["toolcase.webhook.citiccard.com"] = "mutated"
		fmt.Println(patches)
		return patches, nil
	}
	return nil, nil
}
func main() {
	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)
	toolConfig := config.NewToolConfig()
	mux := http.NewServeMux()
	mux.Handle("/mutate", admitFuncHandler(applyToolConfig, &toolConfig))
	server := &http.Server{
		// We listen on port 8443 such that we do not need root privileges or extra capabilities for this server.
		// The Service object will take care of mapping this port to the HTTPS port 443.
		Addr:    ":8443",
		Handler: mux,
	}
	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}
