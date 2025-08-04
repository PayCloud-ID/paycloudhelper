module bitbucket.org/paycloudid/paycloudhelper

go 1.23.0

toolchain go1.24.3

require (
	dario.cat/mergo v1.0.2
	github.com/getsentry/sentry-go v0.33.0
	github.com/joho/godotenv v1.5.1
	github.com/kataras/golog v0.1.12
	github.com/kataras/pio v0.0.13
	github.com/rabbitmq/amqp091-go v1.10.0
)

require (
	github.com/bytedance/sonic v1.14.0 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/json-iterator/go v1.1.12
	github.com/labstack/echo/v4 v4.11.3
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/thedevsaddam/govalidator v1.9.10
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)

retract (
	//bug in audit trail unsafe push, should be safe
	v1.6.0
	//bug in InitializeApp
	v1.5.2
)
