// Command migrate applies or rolls back database migrations.
//
// Usage:
//
//	migrate up
//	migrate down
//	migrate version
//	migrate force <v>
//
// The DSN is read from POSTGRES_DSN, falling back to the config default.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"oriva/backend-go/config"
	"oriva/backend-go/repositories/postgres"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	k := koanf.New(".")
	_ = k.Load(rawbytes.Provider(config.DefaultConfig), yaml.Parser())
	var cfg config.Config
	_ = k.Unmarshal("", &cfg)
	return cfg.Postgres.DSN
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate up|down|version|force <v>")
	}

	d := dsn()
	switch os.Args[1] {
	case "up":
		must(postgres.RunUp(d))
		fmt.Println("migrations applied")
	case "down":
		must(postgres.RunDown(d))
		fmt.Println("migrations rolled back")
	case "version":
		v, dirty, err := postgres.Version(d)
		must(err)
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
	case "force":
		if len(os.Args) < 3 {
			log.Fatal("usage: migrate force <v>")
		}
		v, err := strconv.Atoi(os.Args[2])
		must(err)
		must(postgres.Force(d, v))
		fmt.Printf("forced to version %d\n", v)
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
