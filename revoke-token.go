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

func logVerifyTokenErr(ctx echo.Context, info string) {
	LogE("%s => %s", ctx.Request().RequestURI, info)
}

func RevokeToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var response ResponseApi
		// get token
		tokenStr := ctx.Request().Header.Get("Authorization")

		tokens := strings.Split(tokenStr, " ")
		if len(tokens) != 2 {
			logVerifyTokenErr(ctx, "error : invalid authorization token")
			response.Unauthorized("invalid authorization token", "")
			return ctx.JSON(response.Code, response)
		}

		if tokens[0] != "Bearer" {
			logVerifyTokenErr(ctx, "error : authorization token type does not match")
			response.Unauthorized("authorization token type does not match", "")
			return ctx.JSON(response.Code, response)
		}

		var token *jwt.Token
		token, err := jwt.Parse(tokens[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				LogW("middlewares.GetTokenClaims() token : %v", token.Method.(*jwt.SigningMethodRSA))
				return nil, errors.New("invalid signing method")
			}
			pbKey := os.Getenv("APP_PUBLIC_KEY")
			return parsePublicKey([]byte(pbKey))
		})

		if err != nil {
			logVerifyTokenErr(ctx, "error : authorization token credentials do not match")
			response.Unauthorized("authorization token credentials do not match", "")
			return ctx.JSON(response.Code, response)
		}

		if !token.Valid {
			logVerifyTokenErr(ctx, "error : invalid authorization token credentials")
			response.Unauthorized("invalid authorization token credentials", "")
			return ctx.JSON(response.Code, response)
		}

		tokenClaim, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			logVerifyTokenErr(ctx, "error : token claims is not valid")
			response.Unauthorized("token claims is not valid", "")
			return ctx.JSON(response.Code, response)
		}

		timeData, _ := time.Parse("2006-01-02 15:04:05", tokenClaim["Expired"].(string))
		currentTime := time.Now()
		if currentTime.After(timeData) {
			logVerifyTokenErr(ctx, "error : authorization token has expired")
			response.Unauthorized("authorization token has expired", "")
			return ctx.JSON(response.Code, response)
		}
		merchantId, ok := tokenClaim["MerchantId"].(float64)
		if !ok {
			response.Unauthorized("invalid authorization token merchant", "")
			return ctx.JSON(response.Code, response)
		}
		// get redis key revoke token
		var value revokeToken
		key := "revoke_token_" + strconv.Itoa(int(merchantId))
		data, err := GetRedis(key)
		if err != nil {
			if strings.Contains(err.Error(), "redis: nil") {
				return next(ctx)
			}
			LoggerErrorHub(err)
			response.InternalServerError(err)
			return ctx.JSON(response.Code, response)
		}
		// if key exist, return revoke token
		err = jsoniter.ConfigFastest.Unmarshal([]byte(data), &value)
		if err != nil {
			LoggerErrorHub(err)
			response.InternalServerError(err)
			return ctx.JSON(response.Code, response)
		}

		if _, ok := listStatusRevoke[value.Status]; ok {
			LogW("revoke token: merchant has been ", listStatusRevoke[value.Status])
			response.Unauthorized("revoke jwt token", strconv.Itoa(value.Status))
			return ctx.JSON(response.Code, response)
		}

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
