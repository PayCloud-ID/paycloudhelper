package paycloudhelper

import "github.com/PayCloud-ID/paycloudhelper/phhelper"

func buildLogPrefix(functionName string) string {
	return phhelper.BuildLogPrefix(functionName)
}
