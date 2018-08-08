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


func mergeEnv(oldEnv []string, newEnv string, force, debug bool) (env []string, err error){
	envDic := getEnvDict(oldEnv)
	for _, env := range strings.Split(newEnv, "|") {
		if env == "" {
			continue
		}
		if debug {
			logrus.Infof("HOUDINI: Found env '%s'", env)
		}
		slc := strings.Split(env, "=")
		if len(slc) == 2 {
			if _, ok := envDic[slc[0]]; !ok {
				logrus.Infof("HOUDINI: Add env '%s'", env)
				envDic[slc[0]] = slc[1]
			} else {
				if force {
					logrus.Infof("HOUDINI: ENV key '%s' already set; overwritten with '%s' (due to force-environment is enabled)", slc[0], env)
					envDic[slc[0]] = slc[1]
				} else {
					logrus.Infof("HOUDINI: ENV  '%s' already set (%s), skipping new value '%s' (due to force-environment is disabled)", slc[0], env, slc[1])
				}
			}
		}
	}
	for k,v := range envDic {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env, err
}

func getEnvDict(env []string) map[string]string {
	envDic := map[string]string{}
	for _, e := range env {
		slc := strings.Split(e, "=")
		if len(slc) == 2 {
			envDic[slc[0]] = slc[1]
		}
	}
	return envDic
}

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
	// force houdini
	forceHoudini, _ := c.BoolOr("default.force-houdini", false)
	// check for debug flag
	debugHoudini, _ := c.BoolOr("default.debug", false)
	triggerLabel, err := c.StringOr("default.trigger-label", "houdini")
	if err != nil {
		logrus.Infof("HOUDINI: %s", err.Error())
	}
	vLabel, okLabel := params.Config.Labels[triggerLabel]
	triggerEnv, err := c.StringOr("default.trigger-env", "HOUDINI_ENABLED")
	envDic := getEnvDict(params.Config.Env)
	vEnv, okEnv := envDic[triggerEnv]
	triggerGpuLabel, err := c.StringOr("gpu.trigger-label", "houdini-gpu-enabled")
	if err != nil {
		logrus.Infof("HOUDINI: %s", err.Error())
	}
	vGpuLabel, okGpuLabel := params.Config.Labels[triggerGpuLabel]
	triggerGpuEnv, err := c.StringOr("gpu.trigger-env", "HOUDINI_GPU_ENABLED")
	vGpuEnv, okGpuEnv := envDic[triggerGpuEnv]
	vNvEnv, okNvEnv := envDic["NVIDIA_VISIBLE_DEVICES"]
	triggerPrivilegedEnv, err := c.StringOr("container.privileged-trigger-env", "HOUDINI_CONTAINER_PRIVILEGED")
	vPrivEnv, okPrivEnv := envDic[triggerPrivilegedEnv]
	switch {
	case forceHoudini:
		logrus.Infof("HOUDINI: Force houdini on all containers")
	// ENV is set
	case okEnv && vEnv == "true":
		logrus.Infof("HOUDINI: Trigger houdini, since %s==true", vEnv)
	case okGpuEnv && vGpuEnv == "true":
		logrus.Infof("HOUDINI: Trigger houdini, since %s==true", triggerGpuEnv)
	// NVIDIA_VISIBLE_DEVICES is set
	case okNvEnv:
		logrus.Infof("HOUDINI: Trigger houdini, since NVIDIA_VISIBLE_DEVICES==%s", vNvEnv)
	// Labels are set
	case okLabel && vLabel == "true":
		logrus.Infof("HOUDINI: Trigger houdini, as label '%s' is 'true'.", triggerLabel)
	case okGpuLabel && vGpuLabel == "true":
		logrus.Infof("HOUDINI: Trigger houdini, as label '%s' is 'true'.", triggerGpuLabel)
	case okPrivEnv && vPrivEnv == "true":
		logrus.Infof("HOUDINI: Trigger houdini, as env %s==true", triggerPrivilegedEnv)
	default:
		logrus.Infof("HOUDINI: Skip Houdini, since labels and env do not trigger the patch.")
		if debugHoudini {
			logrus.Infof("HOUDINI: Labels: %v", params.Config.Labels)
			logrus.Infof("HOUDINI: Env: %v", params.Config.Env)
		}
		return params, nil
	}
	/////// Containers
	// Privileged containers
	switch {
	case okPrivEnv && vPrivEnv == "true":
		logrus.Infof("HOUDINI: Set privileged mode, since %s==true", triggerPrivilegedEnv)
		params.HostConfig.Privileged = true
	}
	// remove the container automatically
	//// In case it is used within docker build it needs to be disabled
	// TODO: Heuristic is flawed, but if Logdriver is none it might be docker build
	isBuilding := params.HostConfig.LogConfig.Type == "none"

	cntRmLabel, _ := c.StringOr("container.remove-label", "houdini.container.remove")
	v, ok := params.Config.Labels[cntRmLabel]
	if ok {
		if ! isBuilding {
			logrus.Infof("HOUDINI: set '--rm' flag according to '%s=%s'", cntRmLabel, v)
			params.HostConfig.AutoRemove = v == "true"
		} else {
			logrus.Infof("HOUDINI: skip AutoRemove piece as it seems docker build is used")
		}


	} else if b, _ := c.BoolOr("container.remove", false);b {
		if ! isBuilding {
			logrus.Infof("HOUDINI: remove container when finished")
			params.HostConfig.AutoRemove = true
		} else {
			logrus.Infof("HOUDINI: skip AutoRemove piece as it seems docker build is used")
		}
	}


	// USER
	user := ""
	keepUserLabel, _ := c.StringOr("user.keep-user-label", "houdini.user.keep")
	v, ok = params.Config.Labels[keepUserLabel]
	if ok && v == "true" {
		logrus.Infof("HOUDINI: Keep the user as '%s==true'", keepUserLabel)

	} else {
		uMode, _ := c.StringOr("user.mode", "default")
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
	forceEnv, _ := c.BoolOr("default.force-environment", false)
	params.Config.Env, _ = mergeEnv(params.Config.Env, envs, forceEnv, debugHoudini)
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
	// DEVICES
	devSet := set.New()
	devs, err := c.StringOr("default.devices", "")
	for _, dev := range strings.Split(devs, ",") {
		if dev == "" {
			continue
		}
		if _, err := os.Stat(dev); err == nil {
			devSet.Add(dev)
		}
	}
	//////////////// GPU
	switch {
	case okGpuEnv && vGpuEnv == "true":
		logrus.Infof("HOUDINI: Add GPU, as env '%s' is 'true'.", triggerGpuEnv)
	case okGpuLabel && vGpuLabel == "true":
		logrus.Infof("HOUDINI: Add GPU, as label '%s' is 'true'.", triggerGpuLabel)
	default:
		logrus.Infof("HOUDINI: Skip GPU, as label '%s' nor env '%s' are not 'true'.", triggerGpuLabel, triggerGpuEnv)
		if len(devSet.List()) != 0 {
			params.HostConfig.Devices = evalDevMap(devSet)
		}
		return params, nil
	}
	mnts, err = c.StringOr("gpu.mounts", "")
	if err != nil {
		logrus.Errorf("HOUDINI: %s", err.Error())
	}
	for _, m := range strings.Split(mnts, ",") {
		if m == "" {
			continue
		}
		logrus.Infof("HOUDINI: Add GPU bind mount '%s'", m)
		params.HostConfig.Binds = append(params.HostConfig.Binds, m)
	}
	// GPU-ENV
	envs, err = c.StringOr("gpu.environment", "")
	forceEnv, _ = c.BoolOr("gpu.force-environment", false)
	params.Config.Env, _ = mergeEnv(params.Config.Env, envs, forceEnv, debugHoudini)
	// CUDA libs
	cfiles, _ := c.StringOr("gpu.cuda-files", "libcuda")
	cdirs, err := c.String("gpu.cuda-libpath")
	if err != nil || cdirs == "" {
		logrus.Infof("HOUDINI: gpu.cuda-libpath is empty - skip")
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
	// Add NVIDIA_VISIBLE_DEVICES
	devSet, err = evalDevices(params.Config.Env, devSet)
	params.HostConfig.Devices = evalDevMap(devSet)
	return params, nil
}

func evalDevMap(devSet *set.Set) (devMap []container.DeviceMapping) {
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
	return devMap
}