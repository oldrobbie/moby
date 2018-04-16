package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"strings"
	"github.com/docker/docker/api/types"
	"github.com/zpatrick/go-config"
	"github.com/sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"os"
	"path/filepath"
	"fmt"
)

func getCudaSO(p string) []string {
	list := make([]string, 0, 10)
	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path),"libcuda") {
			list = append(list, path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("walk error [%v]\n", err)
	}
	return list
}

func HoudiniChanges(c *config.Config, params types.ContainerCreateConfig) (types.ContainerCreateConfig, error) {
	// USER
	uMode, _ := c.StringOr("user.mode", "static")
	user := ""
	switch uMode {
	case "static":
		user, _ = c.StringOr("default.user", "")
	default:
		logrus.Errorf("HOUDINI: Unkown user-mode '%s'", uMode)
	}
	if user != "" {
		logrus.Infof("HOUDINI: Overwrite user '%s' with '%s'", params.Config.User, user)
		params.Config.User = user
	}
	// MOUNTS
	mnts, err := c.StringOr("default.mounts", "")
	if err != nil {
		logrus.Errorf("HOUDINI: %s", err.Error())
	}
	for _, m := range strings.Split(mnts, ",") {
		if m == "" {
			continue
		}
		logrus.Infof("HOUDINI: Add bind '%s", m)
		params.HostConfig.Binds = append(params.HostConfig.Binds, m)
	}
	// CUDA libs
	cdirs, err := c.StringOr("cuda.libcuda", "/usr/lib/x86_64-linux-gnu/")
	for _, cdir := range strings.Split(cdirs, ",") {
		cudaSoFiles := getCudaSO(cdir)
		for _, cFile := range cudaSoFiles {
			m := fmt.Sprintf("%s:%s", cFile, cFile)
			logrus.Infof("HOUDINI: Add cuda library '%s", m)
			params.HostConfig.Binds = append(params.HostConfig.Binds, m)
		}
	}
	// DEVICES
	devs, err := c.StringOr("default.devices", "")
	for _, dev := range strings.Split(devs, ",") {
		if dev == "" {
			continue
		}
		if _, err := os.Stat(dev); err == nil {
			dm := container.DeviceMapping{
				PathOnHost: dev,
				PathInContainer: dev,
				CgroupPermissions: "rwm",
			}
			logrus.Infof("HOUDINI: Add device '%s", dev)
			params.HostConfig.Devices = append(params.HostConfig.Devices, dm)
		}
	}

	envs, err := c.StringOr("default.environment", "")
	for _, env := range strings.Split(envs, ",") {
		if env == "" {
			continue
		}
		logrus.Infof("HOUDINI: Add env '%s", env)
		params.Config.Env = append(params.Config.Env, env)
	}

	return params, nil
}
