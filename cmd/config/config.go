package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Tool struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type toolsList []Tool
type ToolConfig struct {
	Image string `json:"image"`
	Tools toolsList `json:"tools"`
	dist map[string]Tool
}

const (
	InitcontainersTemplate = `{"command": "CMD","image": "IMAGE","name": "tool", "volumeMounts": [{"mountPath": "/tools","name": "tool-volume"}]}`
	EMPTYDIR_TAMPLATE      = `{"emptyDir": {},"name": "tool-volume"}`
	VOLUMEMOUNTS_TEMPLATE  = `"volumeMounts": [{"mountPath": "/tools","name": "tool-volume"}]`
)

func (me *ToolConfig) GetTool(name string) (Tool) {
	v := me.dist[name]
	return v
}
func NewToolConfig() ToolConfig {
	data, err := ioutil.ReadFile("/etc/tool/tool.config")
	if err != nil {
		log.Fatal("can not read config file /etc/tool/tool.config: no such file error")
	}
	var config = ToolConfig{
		dist: make(map[string]Tool),
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatal("can not convert the config file /etc/tool/tool.config")
	}
	config.dist = make(map[string]Tool)
	for _, item := range config.Tools {
		config.dist[item.Name] = item
	}
	return config
}

