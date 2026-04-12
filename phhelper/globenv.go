package phhelper

import "sync"

var (
	muGlob      sync.RWMutex
	globAppName string
	globAppEnv  string
)

func GetAppName() string {
	muGlob.RLock()
	defer muGlob.RUnlock()
	return globAppName
}

func GetAppEnv() string {
	muGlob.RLock()
	defer muGlob.RUnlock()
	return globAppEnv
}

func SetAppName(v string) {
	muGlob.Lock()
	defer muGlob.Unlock()
	globAppName = v
}

func SetAppEnv(v string) {
	muGlob.Lock()
	defer muGlob.Unlock()
	globAppEnv = v
}
