package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/zpatrick/go-config"
	"strings"
	"os"
)
const (
	houdiniCfg = "/etc/docker/houdini.ini"
)

func HoudiniChanges(params types.ContainerCreateConfig) (types.ContainerCreateConfig, error) {
	_, err :=  os.Open(houdiniCfg)
	if err != nil {
		return params, nil
	}
	iniFile := config.NewINIFile(houdiniCfg)
	c := config.NewConfig([]config.Provider{iniFile})
	user, _ := c.StringOr("default.user", "")
	if user != "" {
		fmt.Printf(">> Overwrite user '%s' with '%s'\n", params.Config.User, user)
		params.Config.User = user
	}
	mnts, err := c.StringOr("default.mounts", "")
	if err != nil {
		fmt.Printf(">> %s\n", err.Error())
	}
	for _, m := range strings.Split(mnts, ",") {
		if m == "" {
			continue
		}
		fmt.Printf(">> Add bind '%s\n", m)
		params.HostConfig.Binds = append(params.HostConfig.Binds, m)
	}

	return params, nil
}
