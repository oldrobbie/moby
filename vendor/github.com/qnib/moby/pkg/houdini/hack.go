package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"strings"
	"github.com/docker/docker/api/types"
	"github.com/zpatrick/go-config"
	"github.com/sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"os"
	"gopkg.in/fatih/set.v0"
	"path/filepath"
	"fmt"
)

func getCudaSO(p string, cfiles string) []string {
	prefixes := strings.Split(cfiles, ",")
	list := set.New()
	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(filepath.Base(path), prefix) {
				list.Add(path)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("walk error [%v]\n", err)
	}
	res := make([]string, 0, 1)
	for _, x := range list.List() {
		res = append(res, x.(string))
	}
	return res
}

func HoudiniChanges(c *config.Config, params types.ContainerCreateConfig) (types.ContainerCreateConfig, error) {
	// Skip Houdini
	skipLabel, _ := c.StringOr("labels.skip", "com.docker.houdini.skip")
	if v, ok := params.Config.Labels[skipLabel] ; ok {
		if v == "true" {
			logrus.Infof("HOUDINI: Skip HOUDINI changes, as label '%s' is 'true'", skipLabel)
			return params, nil
		}
	}
	// USER
	uMode, _ := c.StringOr("user.mode", "default")
	user := ""
	switch uMode {
	case "default":
		user, _ = c.StringOr("user.default", "")
	case "env":
		key, _ := c.StringOr("user.key", "HOUDINI_USER")
		for _, e := range params.Config.Env {
			kv := strings.Split(e, "=")
			if key == kv[0] {
				user = kv[1]
				logrus.Infof("HOUDINI: Got user '%s' from variable '%s'", user, key)
				break
			}
		}
		if user == "" {
			logrus.Infof("HOUDINI: Could not derive user from container env var '%s', look for user.default", key)
			user, _ = c.StringOr("user.default", "")
		}

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
		logrus.Infof("HOUDINI: Add bind '%s'", m)
		params.HostConfig.Binds = append(params.HostConfig.Binds, m)
	}
	// CUDA libs
	cfiles, _ := c.StringOr("cuda.files", "libcuda")
	cdirs, err := c.StringOr("cuda.libpath", "/usr/lib/x86_64-linux-gnu/")
	logrus.Infof("HOUDINI: Search dirs '%s' for '%s'", cdirs, cfiles)
	if err == nil {
		for _, cdir := range strings.Split(cdirs, ",") {
			logrus.Infof("HOUDINI: Search dir '%s' for '%s'", cdir, cfiles)
			cudaSoFiles := getCudaSO(cdir, cfiles)
			for _, cFile := range cudaSoFiles {
				m := fmt.Sprintf("%s:%s:ro", cFile, cFile)
				logrus.Infof("HOUDINI: Add cuda library '%s:ro", cFile)
				params.HostConfig.Binds = append(params.HostConfig.Binds, m)
			}
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
			logrus.Infof("HOUDINI: Add device '%s'", dev)
			params.HostConfig.Devices = append(params.HostConfig.Devices, dm)
		}
	}

	envs, err := c.StringOr("default.environment", "")
	for _, env := range strings.Split(envs, ",") {
		if env == "" {
			continue
		}
		logrus.Infof("HOUDINI: Add env '%s'", env)
		params.Config.Env = append(params.Config.Env, env)
	}

	return params, nil
}
