module bitbucket.org/paycloudid/paycloudhelper

go 1.24.0

toolchain go1.24.3

require (
	dario.cat/mergo v1.0.2
	github.com/bytedance/sonic v1.15.0
	github.com/getsentry/sentry-go v0.43.0
	github.com/go-redsync/redsync/v4 v4.15.0
	github.com/joho/godotenv v1.5.1
	github.com/kataras/golog v0.1.13
	github.com/kataras/pio v0.0.14
	github.com/rabbitmq/amqp091-go v1.10.0
	golang.org/x/time v0.14.0
)

require (
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	golang.org/x/arch v0.24.0 // indirect
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/json-iterator/go v1.1.12
	github.com/labstack/echo/v4 v4.15.1
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/thedevsaddam/govalidator v1.9.10
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

retract (
	//bug redis error message, should be quiet
	v1.6.3
	//bug in audit trail unsafe push, should be safe
	v1.6.0
	//bug in InitializeApp
	v1.5.2
)
