/*
validate header request
*/

package paycloudhelper

import (
	"github.com/thedevsaddam/govalidator"
)

type Headers struct {
	IdempotencyKey string `json:"idem_key"`
	Session        string `json:"session"`
	Csrf           string `json:"csrf"`
}

func (h *Headers) ValiadateHeaderIdem() interface{} {

	validator := govalidator.New(govalidator.Options{
		Data: h,
		Rules: govalidator.MapData{
			"idem_key": []string{"required", "char_libs", "max:50"},
			"session":  []string{"numeric_null_libs", "max:60"},
		},
		RequiredDefault: true,
	}).ValidateStruct()

	if len(validator) > 0 {
		return validator
	}

	return nil
}

func (h *Headers) ValiadateHeaderCsrf() interface{} {

	validator := govalidator.New(govalidator.Options{
		Data: h,
		Rules: govalidator.MapData{
			"csrf": []string{"required", "char_libs", "max:50"},
		},
		RequiredDefault: true,
	}).ValidateStruct()

	if len(validator) > 0 {
		return validator
	}

	return nil
}
