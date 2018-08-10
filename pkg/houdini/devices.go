package houdini // import "github.com/docker/docker/pkg/houdini"


import (
	"sync"
	"io/ioutil"
	"fmt"
	"os"
	"regexp"
	"time"
	"github.com/docker/docker/container"
)

var (
	devRegex = regexp.MustCompile(`nvidia\d+`)
)

type Resources struct {
	resource string
	reserved 	bool
	start 		time.Time
	lastRelease time.Time
	container 	container.Container
}

func NewResources(res string) Resources {
	fmt.Printf("HOUDINI: New reservation created '%s'\n", res)
	return Resources{reserved: false, resource: res}
}

func (r *Resources) IsReserved() bool {
	return r.reserved
}

func (r *Resources) DoReserved(cnt container.Container) (err error) {
	if r.IsReserved() {
		return fmt.Errorf("%s already booked by '%s'", r.resource, r.container.Name)
	}
	r.reserved = true
	r.container = cnt
	r.start = time.Now()
	return
}

func (r *Resources) Release() (err error) {
	r.reserved = false
	return
}

type DevRegistry struct {
	lock sync.Mutex
	reservation map[string]Resources
}

func (d *DevRegistry) Init() {
	d.lock.Lock()
	defer d.lock.Unlock()
	files, err := ioutil.ReadDir("/dev/")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, f := range files {
		name := f.Name()
		if devRegex.MatchString(name) {
			fmt.Printf("HOUDINI: Found %s\n", name)
			d.reservation[name] = NewResources(name)
		}

	}
}

func GetDevRegistry() DevRegistry {
	dr := DevRegistry{
		reservation: make(map[string]Resources),
	}

	return dr
}
