package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"strings"
	"github.com/docker/docker/api/types"
	"github.com/zpatrick/go-config"
	"github.com/sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	"os"
	"os/user"
	"gopkg.in/fatih/set.v0"
	"path/filepath"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
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

func evalUser(ustr string) (home, uid, gid string, err error) {
	tempUid, err := strconv.Atoi(ustr)
	if err == nil {
		u, err := user.LookupId(string(tempUid))
		if err != nil {
			return "","","", err
		}
		return u.HomeDir, u.Uid, u.Gid, err
	}
	// must be a username
	u, err := user.Lookup(ustr)
	if err != nil {
		return"","","", err
	}
	return u.HomeDir, u.Uid, u.Gid, err
}

func evalDevices(env []string, devSet *set.Set) (ds *set.Set, err error) {
	for _, e := range env {
		slc := strings.Split(e, "=")
		if len(slc) != 2 {
			continue
		}
		if slc[0] == "NVIDIA_VISIBLE_DEVICES" {
			if slc[1] == "all" {
				files, err := ioutil.ReadDir("/dev/")
				if err != nil {
					logrus.Fatal(err)
				}
				for _, f := range files {
					if strings.HasPrefix(f.Name(), "nvidia") {
						match, _ := regexp.MatchString(`nvidia\d+$`, f.Name())
						if ! match {
							continue
						}
						devSet.Add(fmt.Sprintf("/dev/%s", f.Name()))
					}
				}
			} else {
				dev := strings.Split(slc[1], ",")
				for _, d := range dev {
					devSet.Add(fmt.Sprintf("/dev/nvidia%s", d))
				}
			}
		}
		if len(devSet.List()) >= 1 {
			// In case the list contains at least one device, /dev/nvidiactl and (if present) /dev/nvidia-uvm is added
			for _, d := range []string{"nvidia-uvm", "nvidiactl"} {
				devPath := fmt.Sprintf("/dev/%s", d)
				if _, err := os.Stat(devPath); err != nil {
					continue
				}
				devSet.Add(devPath)
			}
		}
	}
	return devSet, err
}

func HoudiniChanges(c *config.Config, params types.ContainerCreateConfig) (types.ContainerCreateConfig, error) {
	// check for label, if not present of false -> SKIP
	triggerLabel, _ := c.StringOr("default.trigger-label", "houdini.enable")
	v, ok := params.Config.Labels[triggerLabel]
	if ok {
		if v != "true" {
			logrus.Infof("HOUDINI: Skip HOUDINI changes, as label '%s' is not 'true'.", triggerLabel)
			return params, nil
		}
	} else {
		logrus.Infof("HOUDINI: Skip HOUDINI changes, as label '%s' is not set.", triggerLabel)
		return params, nil
	}
	// USER
	uMode, _ := c.StringOr("user.mode", "default")
	user := ""
	switch uMode {
	case "static":
		uCfg, err := c.String("user.default")
		if err != nil {
			logrus.Warnln("HOUDINI: user.default is not set!")
		}
		logrus.Infof("HOUDINI: Overwrite user '%s' with '%s'", params.Config.User, uCfg)
		params.Config.User = uCfg
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
		uHome, uid, gid, err := evalUser(user)
		if err != nil || uid == "" {
			logrus.Warnf("HOUDINI: Could not eval user '%s'", user)
		} else {
			uCfg := fmt.Sprintf("%s:%s", uid, gid)
			logrus.Infof("HOUDINI: Overwrite user '%s' with '%s'", params.Config.User, uCfg)
			params.Config.User = uCfg
			if b, _ := c.BoolOr("user.set-home-env", false);b {
				logrus.Infof("HOUDINI: Set '$HOME=%s'", uHome)
				params.Config.Env = append(params.Config.Env, fmt.Sprintf("HOME=%s", uHome))
			}

		}
	}
	// Env
	envs, err := c.StringOr("default.environment", "")
	for _, env := range strings.Split(envs, ",") {
		if env == "" {
			continue
		}
		logrus.Infof("HOUDINI: Add env '%s'", env)
		params.Config.Env = append(params.Config.Env, env)
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
	cdirs, err := c.String("cuda.libpath")
	if err != nil || cdirs == "" {
		logrus.Infof("HOUDINI: cuda.libpath is empty - skip", cdirs, cfiles)
	} else {
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
	}
	devSet := set.New()
	// DEVICES
	devs, err := c.StringOr("default.devices", "")
	for _, dev := range strings.Split(devs, ",") {
		if dev == "" {
			continue
		}
		if _, err := os.Stat(dev); err == nil {
			devSet.Add(dev)
		}
	}
	// Add NVIDIA_VISIBLE_DEVICES
	devSet, err = evalDevices(params.Config.Env, devSet)
	// Eval devmap again
	devMap := []container.DeviceMapping{}
	for _, d := range devSet.List() {
		dev := d.(string)
		logrus.Infof("HOUDINI: Add device '%s'", dev)
		dm := container.DeviceMapping{
			PathOnHost: dev,
			PathInContainer: dev,
			CgroupPermissions: "rwm",
		}
		devMap = append(devMap, dm)
	}
	params.HostConfig.Devices = devMap

	return params, nil
}
