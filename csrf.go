/*
this middleware function is to validate csrf token is exist in redis
if exist continue to process the request
if not exist return error
*/

package paycloudhelper

import (
	"strings"

	"github.com/labstack/echo/v4"
)

func VerifCsrf(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var response ResponseApi

		// Get or generate request ID for tracing
		requestID := GetOrGenerateRequestID(c.Request().Header.Get("X-Request-ID"))
		c.Request().Header.Set("X-Request-ID", requestID)
		c.Response().Header().Set("X-Request-ID", requestID) // Echo response to client

		header := &Headers{
			Csrf:      c.Request().Header.Get("X-Xsrf-Token"),
			RequestID: requestID,
		}
		// validate header request
		validate := header.ValiadateHeaderCsrf()
		if validate != nil {
			LoggerErrorHub("invalid validation")
			LogI("[%s] VerifCsrf: validation failed validation=%s", requestID, JSONEncode(validate))
			response.BadRequest("invalid validation", "")
			return c.JSON(response.Code, response)
		}

		// get token from redis
		_, err := GetRedis("csrf-" + header.Csrf)
		if err != nil {
			// if error redis keys not found, return unathorized
			switch strings.Contains(err.Error(), "redis: nil") {
			case true:
				LoggerErrorHub("token csrf not found")
				LogI("[%s] VerifCsrf: token not found token=%s", requestID, header.Csrf)
				response.Unauthorized("token invalid", "")
				return c.JSON(response.Code, response)
			case false:
				LoggerErrorHub(err)
				LogE("[%s] VerifCsrf: redis error token=%s err=%v", requestID, header.Csrf, err)
				response.InternalServerError(err)
				return c.JSON(response.Code, response)
			}
		}

		LogD("[%s] VerifCsrf: token validated token=%s", requestID, header.Csrf)
		return next(c)
	}
}
