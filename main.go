package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	VERSION = "v0.0.1"
	DATE    string
)

func init() {
	cli.VersionPrinter = versionPrinter
}

func main() {
	app := cli.NewApp()
	app.Author = "Rancher Labs, Inc."
	app.Before = beforeFunc
	app.EnableBashCompletion = true
	app.Name = os.Args[0]
	app.Usage = fmt.Sprintf("migrate RDNS from 0.4.x to 0.5.x(%s)", DATE)
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			EnvVar: "DEBUG",
			Usage:  "used to set debug mode.",
		},
		cli.StringFlag{
			Name:   "src_api_endpoint",
			EnvVar: "src_API_ENDPOINT",
			Usage:  "used to set source api endpoint which needs to be migrated.",
			Value:  "http://127.0.0.1:9333",
		},
		cli.StringFlag{
			Name:   "src_endpoints",
			EnvVar: "SRC_ENDPOINTS",
			Usage:  "used to set source etcd endpoints which needs to be migrated.",
			Value:  "http://127.0.0.1:2379",
		},
		cli.StringFlag{
			Name:   "src_prefix",
			EnvVar: "SRC_PREFIX",
			Usage:  "used to set source etcd prefix which needs to be migrated.",
			Value:  "/rdns",
		},
		cli.StringFlag{
			Name:   "src_domain",
			EnvVar: "SRC_DOMAIN",
			Usage:  "used to set source domain which needs to be migrated.",
			Value:  "lb.rancher.cloud",
		},
		cli.StringFlag{
			Name:   "dst_api_endpoint",
			EnvVar: "DST_API_ENDPOINT",
			Usage:  "used to set destination api endpoint.",
		},
		cli.StringFlag{
			Name:   "dst_domain",
			EnvVar: "DST_DOMAIN",
			Usage:  "used to set destination domain.",
			Value:  "lb.rancher.cloud",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		if err := appMain(ctx); err != nil {
			return err
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func appMain(ctx *cli.Context) error {
	if ctx.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	c, err := NewV2Client(
		strings.Split(ctx.String("src_endpoints"), ","),
		ctx.String("src_api_endpoint"),
		ctx.String("src_prefix"),
		ctx.String("src_domain"),
		ctx.String("dst_api_endpoint"),
		ctx.String("dst_domain"),
	)
	if err != nil {
		return err
	}

	if err := c.MigrateFrozen(); err != nil {
		return err
	}

	return c.MigrateRecords()
}

func beforeFunc(c *cli.Context) error {
	if os.Getuid() != 0 {
		logrus.Fatalf("%s: need to be root", os.Args[0])
	}
	return nil
}

func versionPrinter(c *cli.Context) {
	if _, err := fmt.Fprintf(c.App.Writer, VERSION); err != nil {
		logrus.Error(err)
	}
}
