package app

import (
	"github.com/1Panel-dev/1Panel/agent/utils/docker"
	"github.com/1Panel-dev/1Panel/agent/utils/platform"
)

func Init() {
	if platform.Current() == platform.OSWindows {
		return
	}
	go func() {
		_ = docker.CreateDefaultDockerNetwork()
	}()
}
