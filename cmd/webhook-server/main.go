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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"toolcase.demo.com/admission-webhook-demo/cmd/config"
)

const (
	tlsDir         = `/run/secrets/tls`
	tlsCertFile    = `cert.pem`
	tlsKeyFile     = `key.pem`
	toolAnnotation = `webhook.citiccard.com`
)

var (
	podResource = metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
)

func getPatchItem(op string, path string, val interface{}) patchOperation {
	return patchOperation{
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
	//读取配置文件
	initContainerConfigBytes, cerr := ioutil.ReadFile("./json/initContainerConfig.json")
	if cerr != nil {
		fmt.Errorf("could not read from initContainerConfig.json: %v", cerr)
	}
	initContainerConifg := config.InitContainerConfig{}
	json.Unmarshal(initContainerConfigBytes, &initContainerConifg)
	//获取工具列表
	toolKeyList = getPodEnv(pod)
	//配置volumeMount
	volumeMount := configVolumeMount(
		initContainerConifg.VolumeMount.Name,
		initContainerConifg.VolumeMount.ReadOnly,
		"/tools",
	)
	//配置container
	tmpContainer := configContainer(
		initContainerConifg.Container.Name,
		toolConfig.Image,
		constructCmd(toolConfig, toolKeyList),
		volumeMount,
		corev1.PullPolicy(initContainerConifg.Container.ImagePullPolicy),
	)
	//配置volume
	volume := configVolume(initContainerConifg.Volume.Name)

	var patches []patchOperation
	initContainers := pod.Spec.InitContainers
	if initContainers != nil {
		patches = append(patches, getPatchItem("add", "/spec/initContainers/-", tmpContainer))
	} else {
		patches = append(patches, getPatchItem("add", "/spec/initContainers", []corev1.Container{tmpContainer}))
	}
	patches = append(patches, getPatchItem("add", "/spec/containers/0/volumeMounts/-", volumeMount))
	patches = append(patches, getPatchItem("add", "/spec/volumes/-", volume))
	return patches

}

func configVolumeMount(name string, readOnly bool, mountPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		ReadOnly:  readOnly,
		MountPath: mountPath,
	}
}

func configContainer(name string, image string, command []string, volumeMount corev1.VolumeMount, imagePullPolicy corev1.PullPolicy) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           image,
		Command:         command,
		Resources:       corev1.ResourceRequirements{},
		VolumeMounts:    []corev1.VolumeMount{volumeMount},
		ImagePullPolicy: imagePullPolicy,
	}
}

func configVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func getPodEnv(pod corev1.Pod) []string {
	toolSet := make([]string, 0)
	if pod.Spec.Containers != nil {
		for _, c := range pod.Spec.Containers {
			for _, e := range c.Env {
				if e.Name == "tools" {
					toolSet = strings.Split(e.Value, ",")
				}
			}
			fmt.Printf("Container %v ,toolSet is %v", c.Name, toolSet)
		}
	} else {
		fmt.Printf("There is no container in this pod")
	}
	return toolSet
}

func applyToolConfig(req *v1beta1.AdmissionRequest, toolConfig *config.ToolConfig) ([]patchOperation, error) {

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
