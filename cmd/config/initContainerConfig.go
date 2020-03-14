package config

type InitContainerConfig struct {
	VolumeMount VolumeMount
	Container   Container
	Volume      Volume
}

type Container struct {
	Name            string   `json:"Name"`
	Image           string   `json:"Image"`
	Command         []string `json:"Command"`
	ImagePullPolicy string   `json:"ImagePullPolicy"`
}

type VolumeMount struct {
	Name      string `json:"Name"`
	ReadOnly  bool   `json ReadOnly`
	MountPath string `json MountPath`
}

type Volume struct {
	Name string `json:"Name"`
}
