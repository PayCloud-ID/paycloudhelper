package phhelper

var (
	globAppName string
	globAppEnv  string
)

func GetAppName() string {
	return globAppName
}

func GetAppEnv() string {
	return globAppEnv
}

func SetAppName(v string) {
	globAppName = v
}

func SetAppEnv(v string) {
	globAppEnv = v
}
