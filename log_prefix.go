package paycloudhelper

import "bitbucket.org/paycloudid/paycloudhelper/phhelper"

func buildLogPrefix(functionName string) string {
	return phhelper.BuildLogPrefix(functionName)
}
