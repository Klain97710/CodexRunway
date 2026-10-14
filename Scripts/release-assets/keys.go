package main

import (
	"encoding/base64"
	"errors"
	"flag"
	"github.com/licoy/quosparr/internal/personal"
)

func checkKeys(args []string) error {
	flags := flag.NewFlagSet("check-keys", flag.ContinueOnError)
	file := flags.String("config", "mygo.json", "configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var cfg config
	if err := readJSON(*file, &cfg); err != nil {
		return err
	}
	if cfg.Updates.GitHub != personal.Repository || cfg.Updates.TagPrefix != personal.TagPrefix || cfg.Updates.URL != "" {
		return errors.New("unexpected release channel")
	}
	pub, err := base64.StdEncoding.DecodeString(cfg.Updates.PublicKey)
	if err != nil {
		return errors.New("invalid mygo public key")
	}
	if _, err := signingKey(pub); err != nil {
		return err
	}
	var spark sparkleConfig
	if err := readJSON(*file, &spark); err != nil {
		return err
	}
	_, err = sparkleKey(spark.MacOS.InfoPlist.SUPublicEDKey)
	return err
}
