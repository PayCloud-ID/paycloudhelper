/*
this middleware function is to make sure there is no doouble request with same payload
*/

package paycloudhelper

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/labstack/echo/v4"
)

func VerifIdemKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var response ResponseApi

		// Get or generate request ID for tracing
		requestID := GetOrGenerateRequestID(c.Request().Header.Get("X-Request-ID"))
		c.Request().Header.Set("X-Request-ID", requestID)
		c.Response().Header().Set("X-Request-ID", requestID) // Echo response to client

		// Get request path for better logging context
		requestPath := c.Request().URL.Path
		requestMethod := c.Request().Method

		// get headers
		header := &Headers{
			IdempotencyKey: c.Request().Header.Get("Idempotency-Key"),
			Session:        c.Request().Header.Get("Session"),
			RequestID:      requestID,
		}

		// validate header request
		validate := header.ValiadateHeaderIdem()
		if validate != nil {
			LogE("%s [%s] validation failed method=%s path=%s validation=%s", buildLogPrefix("VerifIdemKey"),
				requestID, requestMethod, requestPath, JSONEncode(validate))
			response.BadRequest("invalid idempotency key format", "IDEM_INVALID_FORMAT")
			return c.JSON(response.Code, response)
		}

		// convert time session to int
		if header.Session == "" || header.Session == "0" {
			header.Session = "9"
		}

		session, err := strconv.Atoi(header.Session)
		if err != nil {
			LogE("%s [%s] invalid session header method=%s path=%s session=%s err=%v", buildLogPrefix("VerifIdemKey"),
				requestID, requestMethod, requestPath, header.Session, err)
			response.BadRequest("invalid session header format", "IDEM_INVALID_SESSION")
			return c.JSON(response.Code, response)
		}

		if session < 4 {
			session = 9
		}

		var request map[string]interface{}

		// Get Body and verify key submitted
		if c.Request().Body != nil {
			var status string
			request, status, err = ReadBody(c, header.IdempotencyKey)
			if err != nil {
				LogE("%s [%s] failed to read body method=%s path=%s key=%s err=%v", buildLogPrefix("VerifIdemKey"),
					requestID, requestMethod, requestPath, header.IdempotencyKey, err)
				response.InternalServerError(err)
				return c.JSON(response.Code, response)
			}

			if status != "" {
				LogE("%s [%s] MD5 validation failed method=%s path=%s key=%s status=%s", buildLogPrefix("VerifIdemKey"),
					requestID, requestMethod, requestPath, header.IdempotencyKey, status)
				response.BadRequest("idempotency key does not match request body", "IDEM_KEY_MISMATCH")
				return c.JSON(response.Code, response)
			}
		}

		// get idempotency from redis
		data, err := GetRedis(header.IdempotencyKey)

		// if key exist, return request data has been submitted and request stopped here
		if data != "" {
			LogI("%s [%s] duplicate request detected method=%s path=%s key=%s", buildLogPrefix("VerifIdemKey"),
				requestID, requestMethod, requestPath, header.IdempotencyKey)
			err = jsoniter.ConfigFastest.Unmarshal([]byte(data), &request)
			if err != nil {
				LogE("%s [%s] failed to unmarshal cached data method=%s path=%s key=%s err=%v", buildLogPrefix("VerifIdemKey"),
					requestID, requestMethod, requestPath, header.IdempotencyKey, err)
				response.InternalServerError(err)
				return c.JSON(response.Code, response)
			}
			response.Accepted(request)
			return c.JSON(response.Code, response)
		}

		// if error redis keys not found, store idem key and request to redis
		if err != nil {
			switch strings.Contains(err.Error(), "redis: nil") {
			case true:
				// Key not found, store new request
				switch request {
				case nil:
					err = StoreRedis(header.IdempotencyKey, header, time.Second*time.Duration(session))
					if err != nil {
						LogE("%s [%s] failed to store header in redis method=%s path=%s key=%s err=%v", buildLogPrefix("VerifIdemKey"),
							requestID, requestMethod, requestPath, header.IdempotencyKey, err)
						response.InternalServerError(err)
						return c.JSON(response.Code, response)
					}
				default:
					err = StoreRedis(header.IdempotencyKey, request, time.Second*time.Duration(session))
					if err != nil {
						LogE("%s [%s] failed to store request in redis method=%s path=%s key=%s err=%v", buildLogPrefix("VerifIdemKey"),
							requestID, requestMethod, requestPath, header.IdempotencyKey, err)
						response.InternalServerError(err)
						return c.JSON(response.Code, response)
					}
				}
				LogD("%s [%s] stored new request method=%s path=%s key=%s ttl=%ds", buildLogPrefix("VerifIdemKey"),
					requestID, requestMethod, requestPath, header.IdempotencyKey, session)
			case false:
				// Redis error (not key not found)
				LogE("%s [%s] redis error method=%s path=%s key=%s err=%v", buildLogPrefix("VerifIdemKey"),
					requestID, requestMethod, requestPath, header.IdempotencyKey, err)
				response.InternalServerError(err)
				return c.JSON(response.Code, response)

			}
		}

		return next(c)
	}
}

// ReadBody read body payload and validate with the key
func ReadBody(c echo.Context, idem string) (map[string]interface{}, string, error) {
	request := map[string]interface{}{}
	var jsonMinify []byte
	var err error
	var body []byte

	// read body payload
	content := c.Request().Header.Get("Content-Type")
	switch content {
	case "application/json":
		body, err = io.ReadAll(c.Request().Body)
		if err != nil {
			return nil, "", err
		}

		// convert body bytes
		err = jsoniter.ConfigFastest.Unmarshal(body, &request)
		if err != nil {
			return nil, "", err
		}

		// Assign Back the request body to echo
		c.Request().Body = io.NopCloser(bytes.NewBuffer(body))
	default:
		if strings.Contains(content, "multipart/form-data") {

			err = c.Bind(&request)
			if err != nil {
				return nil, "", err
			}

			body, err = json.Marshal(request)
			if err != nil {
				return nil, "", err
			}

		}
	}

	// convert body to beauty json
	jsonMinify, err = JsonMinify(body)
	if err != nil {
		return nil, "", err
	}

	// verify key has been submitted is valid md5 or not
	status, err := VerifyMD5(idem, jsonMinify)
	if err != nil {
		return nil, "", err
	}

	if status != "" {
		return nil, status, err
	}

	return request, "", nil

}

// VerifyMD5 generate md5 hash and compare the result with current key submitted
func VerifyMD5(idemKey string, request []byte) (string, error) {

	hash := md5.New()

	_, err := hash.Write(request)
	if err != nil {
		return "", err
	}

	md5Generated := hex.EncodeToString(hash.Sum(nil))

	LogD("%s submitted_key=%s generated_key=%s", buildLogPrefix("VerifyMD5"), idemKey, md5Generated)

	if idemKey != md5Generated {
		LogW("%s key mismatch submitted=%s expected=%s", buildLogPrefix("VerifyMD5"), idemKey, md5Generated)
		return "key not valid", nil
	}

	return "", nil
}
