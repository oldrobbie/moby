package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"strings"
	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
	container2 "github.com/docker/docker/container"
	"os"
	"os/user"
	"gopkg.in/fatih/set.v0"
	"path/filepath"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	hconfig "github.com/zpatrick/go-config"

	"sync"
)

const (
	ENV_GPU_REQ 			= "HOUDINI_GPU_REQUESTED"
	ENV_NV_VISIBLE_DEV 		= "NVIDIA_VISIBLE_DEVICES"
	ENV_HOUDINI_ENABLED		= "HOUDINI_ENABLED"
	ENV_HOUDINI_GPU_ENABLED	= "HOUDINI_GPU_ENABLED"
	ENV_HOUDINI_CNT_PRIV	= "HOUDINI_CONTAINER_PRIVILEGED"
	ENV_HOUDINI_USR			= "HOUDINI_USER"
)


type Houdini struct {
	mu  	sync.Mutex
	config *hconfig.Config
	registry DevRegistry
}

func NewHoudini(c string) (*Houdini,error) {
	reg := GetDevRegistry()
	_, err :=  os.Open(c)
	if err == nil {
		logrus.Infof("Loading Houdini config '%s'", c)
		iniFile := hconfig.NewINIFile(c)
		houdiniCfg := hconfig.NewConfig([]hconfig.Provider{iniFile})
		reg.Init()
		h := &Houdini{config: houdiniCfg,registry: reg}
		return h, nil
	}
	return &Houdini{},fmt.Errorf("Could not load Houdini config '%s'", c)
}


func (h *Houdini) ResourceCheck(cnts []*types.Container) {
	h.mu.Lock()
	defer h.mu.Unlock()
	logrus.Infof("HOUDINI: Start ResourceCheck(%d)", len(cnts))
	reservedCnts := map[string]bool{}
	for _, c := range h.registry.reservation {
		logrus.Infof("HOUDINI: Found reservation for '%s'", c.cntName)
		reservedCnts[c.cntName] = true
	}
	for _, cnt := range cnts {
		name := strings.TrimPrefix(cnt.Names[0], "/")
		logrus.Infof("HOUDINI: Check container %s | %s", name, cnt.ID)
		if _, nameMatch := reservedCnts[name];nameMatch {
			reservedCnts[name] = false
		}
		if _, idMatch := reservedCnts[cnt.ID]; idMatch {
			reservedCnts[cnt.ID] = false

		}
	}
	for key, val := range reservedCnts {
		if val {
			h.registry.CleanResourceByCntName(key)
		}
	}
}

func (h *Houdini) ReleaseGPU(cnt *container2.Container) (err error) {
	h.registry.ReleaseResourceByCnt(cnt)
	return
}

func (h *Houdini) ReqisterGPUS(cntName string, req int) ([]string, error) {
	return h.registry.Request(cntName, req)
}

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

// evalUid() checks for UID:GID settings and splits out the UID (string or integer at this point)
func evalUid(ustr string) (uid string) {
	if strings.Contains(ustr, ":") {
		ugid := strings.SplitN(ustr, ":", 2)
		ustr = ugid[0]
		return evalUid(ustr)
	}
	return ustr
}

func evalUser(ustr string) (home, uid, gid string, err error) {
	// split UID:GID if need be
	ustr = evalUid(ustr)
	// from here it can only be a string or an int
	tempUid, err := strconv.Atoi(ustr)
	if err == nil {
		u, err := user.LookupId(string(tempUid))
		if err != nil {
			return "","","", err
		}
		return u.HomeDir, u.Uid, u.Gid, err
	}

	// must be a username
	logrus.Infof("Lookup user '%s'", ustr)
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
		if slc[0] == ENV_NV_VISIBLE_DEV {
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

func (h *Houdini) HoudiniChanges(params types.ContainerCreateConfig) (types.ContainerCreateConfig, error) {
	// force houdini
	forceHoudini, _ := h.config.BoolOr("default.force-houdini", false)
	// check for debug flag
	debugHoudini, _ := h.config.BoolOr("default.debug", false)
	triggerLabel, err := h.config.StringOr("default.trigger-label", "houdini")
	if err != nil {
		logrus.Infof("HOUDINI: %s", err.Error())
	}
	vLabel, okLabel := params.Config.Labels[triggerLabel]
	triggerEnv, err := h.config.StringOr("default.trigger-env", ENV_HOUDINI_ENABLED)
	envDic := getEnvDict(params.Config.Env)
	vEnv, okEnv := envDic[triggerEnv]
	triggerGpuLabel, err := h.config.StringOr("gpu.trigger-label", "houdini-gpu-enabled")
	if err != nil {
		logrus.Infof("HOUDINI: %s", err.Error())
	}
	vGpuLabel, okGpuLabel := params.Config.Labels[triggerGpuLabel]
	triggerGpuEnv, err := h.config.StringOr("gpu.trigger-env", ENV_HOUDINI_GPU_ENABLED)
	vGpuEnv, okGpuEnv := envDic[triggerGpuEnv]
	vNvEnv, okNvEnv := envDic[ENV_NV_VISIBLE_DEV]
	reqGpu := 0
	vReqGPUEnv, okReqGPUEnv := envDic[ENV_GPU_REQ]
	triggerPrivilegedEnv, err := h.config.StringOr("container.privileged-trigger-env", ENV_HOUDINI_CNT_PRIV)
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
		logrus.Infof("HOUDINI: Trigger houdini, since %s==%s", ENV_NV_VISIBLE_DEV, vNvEnv)
	// HOUDINI_GPU_REQUESTED is set and >0
	case okReqGPUEnv:
		reqGpu, err := strconv.Atoi(vReqGPUEnv)
		if err != nil {
			logrus.Infof("HOUDINI: Tried to parse %s='%s': %s", ENV_GPU_REQ, vReqGPUEnv, err.Error())
			return params, nil
		}
		logrus.Infof("HOUDINI: Service requested '%d' (%s) GPUs", reqGpu, ENV_GPU_REQ)
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

	cntRmLabel, _ := h.config.StringOr("container.remove-label", "houdini.container.remove")
	v, ok := params.Config.Labels[cntRmLabel]
	if ok {
		if ! isBuilding {
			logrus.Infof("HOUDINI: set '--rm' flag according to '%s=%s'", cntRmLabel, v)
			params.HostConfig.AutoRemove = v == "true"
		} else {
			logrus.Infof("HOUDINI: skip AutoRemove piece as it seems docker build is used")
		}


	} else if b, _ := h.config.BoolOr("container.remove", false);b {
		if ! isBuilding {
			logrus.Infof("HOUDINI: remove container when finished")
			params.HostConfig.AutoRemove = true
		} else {
			logrus.Infof("HOUDINI: skip AutoRemove piece as it seems docker build is used")
		}
	}


	// USER
	user := ""
	keepUserLabel, _ := h.config.StringOr("user.keep-user-label", "houdini.user.keep")
	keepUserEnv, _ := h.config.StringOr("user.keep-user-env", "HOUDINI_USER_KEEP")
	kUsrVal, kUsrOK := envDic[keepUserEnv]
	v, ok = params.Config.Labels[keepUserLabel]
	if debugHoudini {
		logrus.Info("HOUDINI: Env> val:%v|ok:%v || Label> val:%s|ok:%v", kUsrVal, kUsrOK, v, ok)
	}
	switch {
	case ok && v == "true":
		logrus.Infof("HOUDINI: Keep the user as '%s==true'", keepUserLabel)
	case kUsrOK && kUsrVal == "true":
		logrus.Infof("HOUDINI: Keep the user as '%s==true'", keepUserEnv)
	default:
		uMode, _ := h.config.StringOr("user.mode", "default")
		switch uMode {
		case "static":
			uCfg, err := h.config.String("user.default")
			if err != nil {
				logrus.Warnln("HOUDINI: user.default is not set!")
			}
			logrus.Infof("HOUDINI: Overwrite user '%s' with '%s'", params.Config.User, uCfg)
			params.Config.User = uCfg
		case "default":
			user, _ = h.config.StringOr("user.default", "")
		case "env":
			key, _ := h.config.StringOr("user.key", ENV_HOUDINI_USR)
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
				user, _ = h.config.StringOr("user.default", "")
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
			if b, _ := h.config.BoolOr("user.set-home-env", false);b {
				logrus.Infof("HOUDINI: Set '$HOME=%s'", uHome)
				params.Config.Env = append(params.Config.Env, fmt.Sprintf("HOME=%s", uHome))
			}

		}
	}

	// Env
	envs, err := h.config.StringOr("default.environment", "")
	forceEnv, _ := h.config.BoolOr("default.force-environment", false)
	params.Config.Env, _ = mergeEnv(params.Config.Env, envs, forceEnv, debugHoudini)
	// MOUNTS
	mnts, err := h.config.StringOr("default.mounts", "")
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
	devs, err := h.config.StringOr("default.devices", "")
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
	case okReqGPUEnv:
		reqGpu, err = strconv.Atoi(vReqGPUEnv)
		if err != nil {
			logrus.Infof("HOUDINI: Tried to parse %s='%s': %s", ENV_GPU_REQ, vReqGPUEnv, err.Error())
			return params, nil
		}
	case okNvEnv:
		logrus.Infof("HOUDINI: Add GPU, since %s==%s", ENV_NV_VISIBLE_DEV, vNvEnv)
	default:
		logrus.Infof("HOUDINI: Skip GPU, as label '%s' nor env '%s' are not 'true'.", triggerGpuLabel, triggerGpuEnv)
		if len(devSet.List()) != 0 {
			params.HostConfig.Devices = evalDevMap(devSet)
		}
		return params, nil
	}
	mnts, err = h.config.StringOr("gpu.mounts", "")
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
	envs, err = h.config.StringOr("gpu.environment", "")
	forceEnv, _ = h.config.BoolOr("gpu.force-environment", false)
	params.Config.Env, _ = mergeEnv(params.Config.Env, envs, forceEnv, debugHoudini)
	// CUDA libs
	cfiles, _ := h.config.StringOr("gpu.cuda-files", "libcuda")
	cdirs, err := h.config.String("gpu.cuda-libpath")
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
	// GPU REQUESTED
	if reqGpu > 0 {
		logrus.Infof("HOUDINI: Container '%s' requested '%d' (%s) GPUs", params.Name, reqGpu, ENV_GPU_REQ)
		resList, err := h.ReqisterGPUS(params.Name, reqGpu)
		if err != nil {
			logrus.Infof("HOUDINI: Error registering GPUs: %s", err.Error())
			return params, err
		} else {
			for _, dev := range resList {
				devSet.Add(dev)
			}
		}

	}
	// Add NVIDIA_VISIBLE_DEVICES
	devSet, err = evalDevices(params.Config.Env, devSet)
	params.HostConfig.Devices = evalDevMap(devSet)
	return params, nil
}

func evalDevList(l []string) (devMap []container.DeviceMapping) {
	for _, dev := range l {
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

func evalDevMap(devSet *set.Set) (devMap []container.DeviceMapping) {
	lst := []string{}
	l := devSet.List()
	for _, d := range l {
		lst = append(lst, d.(string))
	}
	return evalDevList(lst)
}