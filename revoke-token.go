/*
this middleware function is to revoke token jwt if status user blocked or suspend
*/
package paycloudhelper

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	jsoniter "github.com/json-iterator/go"
	"github.com/labstack/echo/v4"
)

var listStatusRevoke = map[int]string{
	3: "blacklist",
	4: "suspend",
	7: "closed",
}

type revokeToken struct {
	Status int `json:"status"`
}

func logVerifyTokenErr(ctx echo.Context, requestID, info string) {
	LogE("%s [%s] uri=%s info=%s", buildLogPrefix("logVerifyTokenErr"), requestID, ctx.Request().RequestURI, info)
}

func RevokeToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var response ResponseApi

		// Get or generate request ID for tracing
		requestID := GetOrGenerateRequestID(ctx.Request().Header.Get("X-Request-ID"))
		ctx.Request().Header.Set("X-Request-ID", requestID)
		ctx.Response().Header().Set("X-Request-ID", requestID) // Echo response to client

		// get token
		tokenStr := ctx.Request().Header.Get("Authorization")

		tokens := strings.Split(tokenStr, " ")
		if len(tokens) != 2 {
			logVerifyTokenErr(ctx, requestID, "error : invalid authorization token")
			response.Unauthorized("invalid authorization token", "")
			return ctx.JSON(response.Code, response)
		}

		if tokens[0] != "Bearer" {
			logVerifyTokenErr(ctx, requestID, "error : authorization token type does not match")
			response.Unauthorized("authorization token type does not match", "")
			return ctx.JSON(response.Code, response)
		}

		var token *jwt.Token
		token, err := jwt.Parse(tokens[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				// Never type-assert token.Method in this branch — non-RSA alg would panic (SEC-2).
				LogW("%s [%s] invalid signing method alg=%s type=%T", buildLogPrefix("RevokeToken"), requestID, token.Method.Alg(), token.Method)
				return nil, errors.New("invalid signing method")
			}
			pbKey := os.Getenv("APP_PUBLIC_KEY")
			return parsePublicKey([]byte(pbKey))
		})

		if err != nil {
			logVerifyTokenErr(ctx, requestID, "error : authorization token credentials do not match")
			response.Unauthorized("authorization token credentials do not match", "")
			return ctx.JSON(response.Code, response)
		}

		if !token.Valid {
			logVerifyTokenErr(ctx, requestID, "error : invalid authorization token credentials")
			response.Unauthorized("invalid authorization token credentials", "")
			return ctx.JSON(response.Code, response)
		}

		tokenClaim, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			logVerifyTokenErr(ctx, requestID, "error : token claims is not valid")
			response.Unauthorized("token claims is not valid", "")
			return ctx.JSON(response.Code, response)
		}

		expiredRaw, ok := tokenClaim["Expired"]
		if !ok {
			logVerifyTokenErr(ctx, requestID, "error : token missing Expired claim")
			response.Unauthorized("invalid authorization token credentials", "")
			return ctx.JSON(response.Code, response)
		}
		expiredStr, ok := expiredRaw.(string)
		if !ok || strings.TrimSpace(expiredStr) == "" {
			logVerifyTokenErr(ctx, requestID, "error : invalid Expired claim type or value")
			response.Unauthorized("invalid authorization token credentials", "")
			return ctx.JSON(response.Code, response)
		}
		timeData, err := time.Parse("2006-01-02 15:04:05", expiredStr)
		if err != nil {
			logVerifyTokenErr(ctx, requestID, "error : invalid Expired claim format")
			response.Unauthorized("invalid authorization token credentials", "")
			return ctx.JSON(response.Code, response)
		}
		currentTime := time.Now()
		if currentTime.After(timeData) {
			logVerifyTokenErr(ctx, requestID, "error : authorization token has expired")
			response.Unauthorized("authorization token has expired", "")
			return ctx.JSON(response.Code, response)
		}
		merchantId, ok := tokenClaim["MerchantId"].(float64)
		if !ok {
			logVerifyTokenErr(ctx, requestID, "error : invalid authorization token merchant")
			response.Unauthorized("invalid authorization token merchant", "")
			return ctx.JSON(response.Code, response)
		}
		// get redis key revoke token
		var value revokeToken
		key := "revoke_token_" + strconv.Itoa(int(merchantId))
		data, err := GetRedis(key)
		if err != nil {
			if strings.Contains(err.Error(), "redis: nil") {
				LogD("%s [%s] no revoke token found merchant=%d", buildLogPrefix("RevokeToken"), requestID, int(merchantId))
				return next(ctx)
			}
			LoggerErrorHub(err)
			LogE("%s [%s] redis error merchant=%d err=%v", buildLogPrefix("RevokeToken"), requestID, int(merchantId), err)
			response.InternalServerError(err)
			return ctx.JSON(response.Code, response)
		}
		// if key exist, return revoke token
		err = jsoniter.ConfigFastest.Unmarshal([]byte(data), &value)
		if err != nil {
			LoggerErrorHub(err)
			LogE("%s [%s] failed to unmarshal revoke data merchant=%d err=%v", buildLogPrefix("RevokeToken"), requestID, int(merchantId), err)
			response.InternalServerError(err)
			return ctx.JSON(response.Code, response)
		}

		if _, ok := listStatusRevoke[value.Status]; ok {
			LogW("%s [%s] merchant has been %s merchant=%d status=%d", buildLogPrefix("RevokeToken"), requestID, listStatusRevoke[value.Status], int(merchantId), value.Status)
			response.Unauthorized("revoke jwt token", strconv.Itoa(value.Status))
			return ctx.JSON(response.Code, response)
		}

		LogD("%s [%s] token validated merchant=%d", buildLogPrefix("RevokeToken"), requestID, int(merchantId))
		return next(ctx)
	}
}

func parsePublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("ssh: no key found")
	}

	var rawkey interface{}
	switch block.Type {
	case "PUBLIC KEY":
		parsedPubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rawkey = parsedPubKey
	default:
		return nil, fmt.Errorf("ssh: unsupported key type %q", block.Type)
	}

	switch t := rawkey.(type) {
	case *rsa.PublicKey:
		return t, nil
	default:
		return nil, fmt.Errorf("ssh: unsupported key type %T", rawkey)
	}
}
