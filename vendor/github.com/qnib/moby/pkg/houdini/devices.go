package houdini // import "github.com/docker/docker/pkg/houdini"


import (
	"sync"
	"io/ioutil"
	"fmt"
	"os"
	"regexp"
	"time"
		"strings"
	"github.com/sirupsen/logrus"
	"path"
)

var (
	devRegex = regexp.MustCompile(`nvidia\d+`)
)

type Resources struct {
	resource string
	reserved 	bool
	start 		time.Time
	lastRelease time.Time
	cntName 	string
}

func NewResources(res string) Resources {
	fmt.Printf("HOUDINI: New reservation created '%s'\n", res)
	return Resources{reserved: false, resource: res}
}

func (r *Resources) IsReserved() bool {
	return r.reserved
}

func (r *Resources) DoReserved(cntName string) (err error) {
	if r.IsReserved() {
		return fmt.Errorf("HOUDINI: %s already booked by '%s'", r.resource, r.cntName)
	}
	fmt.Printf("HOUDINI: Reserved %s for %s\n", r.resource, cntName)
	r.reserved = true
	r.cntName = cntName
	r.start = time.Now()
	return
}

func (r *Resources) Release() (err error) {
	r.reserved = false
	return
}

type DevRegistry struct {
	devPath string
	lock sync.Mutex
	reservation map[string]Resources
}

func (dr *DevRegistry) Init() {
	dr.lock.Lock()
	defer dr.lock.Unlock()
	files, err := ioutil.ReadDir(dr.devPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, f := range files {
		name := f.Name()
		if devRegex.MatchString(name) {
			rKey := path.Join(dr.devPath, name)
			logrus.Infof("HOUDINI: Found %s\n", rKey)
			dr.reservation[rKey] = NewResources(name)
		}

	}
}

func GetDevRegistry() DevRegistry {
	dr := DevRegistry{
		devPath: "/dev/",
		reservation: make(map[string]Resources),
	}

	return dr
}
func (dr *DevRegistry) ReleaseResource(key string) {
	logrus.Infof("HOUDINI: Start  ReleaseResource(%s)", key)
	for k, v := range dr.reservation {
		if key == v.cntName {
			v.Release()
			logrus.Infof("HOUDINI: Released %s reserved for container %s\n", k, key)
			dr.reservation[k] = v
		}
	}
}

func (dr *DevRegistry) Deregister(resList []string) {
	for _, ele := range resList {
		dr.ReleaseResource(ele)
	}
}

func (dr *DevRegistry) Request(cntName string, count int) (resList []string, err error) {
	dr.lock.Lock()
	defer dr.lock.Unlock()
	logrus.Infof("StartGPU Request for '%s' w/ count=%d",cntName, count)
	for k, v := range dr.reservation {
		if v.IsReserved() {
			continue
		}
		err = v.DoReserved(cntName)
		if err != nil {
			dr.Deregister(resList)
			return []string{}, fmt.Errorf("Error occured while registering GPU: %s", err.Error())
		}
		resList = append(resList, k)
		count--
		if count == 0 {
			return resList, err
		}
	}
	dr.Deregister(resList)
	return []string{}, fmt.Errorf("Unable to reserve requested amount of GPUs (reqested: %d, granted: %s)", count, strings.Join(resList, ",") )

}