package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	dr := GetDevRegistry()
	dr.devPath = "./testDevs"
	dr.Init()
	devList := []string{}
	for k, _ := range dr.reservation {
		devList = append(devList, k)
	}
	exp := []string{"testDevs/nvidia0","testDevs/nvidia1"}
	assert.Equal(t, exp, devList)
}

func TestDevRegistry_Request(t *testing.T) {
	dr := GetDevRegistry()
	dr.devPath = "./testDevs"
	dr.Init()
	resList, err := dr.Request("testCnt1", 1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"testDevs/nvidia0"}, resList)
	resList, err = dr.Request("testCnt2", 1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"testDevs/nvidia1"}, resList)
	_, err = dr.Request("testCnt3", 1)
	assert.Error(t, err)
	dr.Deregister([]string{"testDevs/nvidia1"})
	resList, err = dr.Request("testCnt4", 1)
	assert.NoError(t, err)
	assert.Equal(t, []string{"testDevs/nvidia1"}, resList)
}

func TestDevRegistry_CleanResourceByCntName(t *testing.T) {
	dr := GetDevRegistry()
	dr.devPath = "./testDevs"
	dr.Init()
	resList, err := dr.Request("testCnt1", 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resList))
	assert.Equal(t, 1, len(dr.GetReservedResources()))
	dr.CleanResourceByCntName("testCnt1")
	assert.Equal(t, 0, len(dr.GetReservedResources()))
}