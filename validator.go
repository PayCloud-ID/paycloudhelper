package qoinhubhelper

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/thedevsaddam/govalidator"
)

const (
	Numeric string = "^-?[0-9]+$"
	Key     string = "^[-a-zA-Z0-9_-]+$"
)

var (
	regexNumeric = regexp.MustCompile(Numeric)
	regexKey     = regexp.MustCompile(Key)
)

func AddValidatorLibs() {
	govalidator.AddCustomRule("numeric_null_libs", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		if str == "" {
			return nil
		}

		err := fmt.Errorf("the %s field must be a valid numeric", field)
		if message != "" {
			err = errors.New(message)
		}

		if !isNumeric(str) {
			return err
		}

		return nil
	})
	govalidator.AddCustomRule("char_libs", func(field string, rule string, message string, value interface{}) error {
		str := toString(value)
		if str == "" {
			return nil
		}

		err := fmt.Errorf("the %s field must be a contains alpha numeric, space, dot, comma, underscore, dash, slash and brackets", field)
		if message != "" {
			err = errors.New(message)
		}

		if !isIdemKey(str) {
			return err
		}

		return nil
	})
}

func toString(v interface{}) string {
	str, ok := v.(string)
	if !ok {
		str = fmt.Sprintf("%v", v)
	}
	return str
}

func isNumeric(str string) bool {
	return regexNumeric.MatchString(str)
}

func isIdemKey(str string) bool {
	return regexKey.MatchString(str)
}
