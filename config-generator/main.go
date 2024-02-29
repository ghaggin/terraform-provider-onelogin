package main

import (
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
)

type config struct {
	subdomain    string
	clientID     string
	clientSecret string

	outputDir string
	resources []string
}

func validateConfig(c *config) (bool, error) {
	valid := true
	errMsg := ""

	if c.subdomain == "" {
		valid = false
		errMsg += "	SUBDOMAIN env variable is required\n"
	}
	if c.clientID == "" {
		valid = false
		errMsg += "	CLIENT_ID env variable is required\n"
	}
	if c.clientSecret == "" {
		valid = false
		errMsg += "	CLIENT_SECRET env variable is required\n"
	}
	if c.outputDir == "" {
		os.Stderr.WriteString("Output directory not specified, using default: ./tmp\n")
		c.outputDir = "./tmp"
	}
	if len(c.resources) == 0 {
		valid = false
		errMsg += "	At least one resource must be specified\n"
	} else {
		validResources := map[string]bool{"roles": true, "mappings": true}
		for _, r := range c.resources {
			if _, ok := validResources[r]; !ok {
				valid = false
				errMsg += "	Invalid resource specified: " + r + "\n"
			}
		}

	}

	if !valid {
		errMsg = "Invalid configuration:\n" + errMsg
		return false, errors.New(errMsg)
	}

	return true, nil
}

func main() {
	config := &config{}
	var resources string
	flag.StringVar(&config.outputDir, "output_dir", "", "Output directory for the generated files")
	flag.StringVar(&resources, "resources", "", "Comma separated list of resources to generate, e.g. roles,mappings,apps")
	flag.Parse()

	config.subdomain = os.Getenv("SUBDOMAIN")
	config.clientID = os.Getenv("CLIENT_ID")
	config.clientSecret = os.Getenv("CLIENT_SECRET")
	config.resources = strings.Split(resources, ",")

	valid, err := validateConfig(config)
	if !valid {
		os.Stderr.WriteString(err.Error() + "\n")
		flag.Usage()
		os.Exit(1)
	}

	client, err := onelogin.NewClient(&onelogin.ClientConfig{
		Subdomain:    config.subdomain,
		ClientID:     config.clientID,
		ClientSecret: config.clientSecret,
	})

	if err != nil {
		os.Stderr.WriteString("Error creating client:")
		os.Stderr.WriteString("  " + err.Error() + "\n")
		os.Exit(2)
	}

	err = NewGenerator(client).Run(config.outputDir, config.resources)
	if err != nil {
		os.Stderr.WriteString("Error running generator:")
		os.Stderr.WriteString("  " + err.Error() + "\n")
		os.Exit(3)
	}
}
