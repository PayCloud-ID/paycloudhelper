package paycloudhelper

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/PayCloud-ID/paycloudhelper/pcauth"
	"github.com/PayCloud-ID/paycloudhelper/phhelper"

	"github.com/joho/godotenv"
)

func init() {
	AddValidatorLibs()
	InitializeLogger()
	InitializeApp()
}

// findEnvPath returns a path to .env by trying, in order: ENV_FILE/DOTENV_PATH
// overrides, CWD, parent dirs, executable dir, then the common container mount
// targets /app/.env and /.env. Empty string means no .env was found. Used so
// apps find .env when run from an IDE, a subdir, a different cwd, or a container
// where the binary lives outside the secret mount directory.
func findEnvPath() string {
	// Explicit operator overrides win. Both names are accepted so services that
	// standardized on either ENV_FILE or DOTENV_PATH keep working.
	for _, key := range []string{"ENV_FILE", "DOTENV_PATH"} {
		if p := os.Getenv(key); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		p := filepath.Join(wd, ".env")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		dir := wd
		for i := 0; i < 5; i++ {
			dir = filepath.Dir(dir)
			if dir == filepath.Dir(dir) {
				break
			}
			p := filepath.Join(dir, ".env")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), ".env")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Common container secret mount targets, checked last so local dev (cwd) wins.
	for _, p := range []string{"/app/.env", "/.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func InitializeApp() {
	envPath := findEnvPath()
	if envPath != "" {
		if err := godotenv.Load(envPath); err != nil {
			LogD("%s failed to load .env from %s err=%v", buildLogPrefix("InitializeApp"), envPath, err)
		}
	} else {
		if err := godotenv.Load(); err != nil {
			LogD("%s .env not found (tried CWD and parents) err=%v", buildLogPrefix("InitializeApp"), err)
		}
	}

	pcauth.ConfigureCostProvider(func() int {
		bcryptCost := pcauth.DefaultCost
		if rawCost := os.Getenv("BCRYPT_COST"); rawCost != "" {
			if parsedCost, err := strconv.Atoi(rawCost); err == nil {
				bcryptCost = parsedCost
			}
		}
		return bcryptCost
	})

	if appName := os.Getenv("APP_NAME"); appName != "" {
		phhelper.SetAppName(appName)
	}
	// APP_ENV is canonical; APP_MODE is accepted as fallback (e.g. develop server .env)
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		phhelper.SetAppEnv(appEnv)
	} else if appMode := os.Getenv("APP_MODE"); appMode != "" {
		phhelper.SetAppEnv(appMode)
	}

	// Validate configuration and log warnings
	LogConfigurationWarnings()
}

func GetAppName() string {
	return phhelper.GetAppName()
}

func GetAppEnv() string {
	return phhelper.GetAppEnv()
}

func SetAppName(v string) {
	phhelper.SetAppName(v)
}

func SetAppEnv(v string) {
	phhelper.SetAppEnv(v)
}
