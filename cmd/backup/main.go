package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/backup"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "manifest":
		create(os.Args[2:])
	case "verify":
		verify(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func create(args []string) {
	flags := flag.NewFlagSet("manifest", flag.ExitOnError)
	artifact := flags.String("artifact", "", "dump artifact path")
	output := flags.String("output", "", "manifest output path")
	dialect := flags.String("dialect", "postgres", "postgres, mysql, or mariadb")
	_ = flags.Parse(args)
	manifest, err := backup.CreateManifest(*artifact, *dialect, time.Now().UTC())
	if err == nil {
		err = backup.WriteManifest(*output, manifest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(*output)
}
func verify(args []string) {
	flags := flag.NewFlagSet("verify", flag.ExitOnError)
	input := flags.String("manifest", "", "manifest path")
	_ = flags.Parse(args)
	manifest, err := backup.ReadManifest(*input)
	if err == nil {
		err = backup.Verify(manifest)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("backup verified")
}
func usage() {
	fmt.Fprintln(os.Stderr, "usage: backup manifest -artifact dump.sql -output dump.manifest.json -dialect postgres | backup verify -manifest dump.manifest.json")
}
