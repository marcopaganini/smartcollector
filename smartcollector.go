// SmartThings sensor data to prometheus gateway
//
// This is a simple SmartThing to Prometheus collector. It uses the textfile collector
// capabilities of the prometheus node exporter to generate interesting data about sensors
// in your SmartThings location.
//
// Check the README.md for installation instructions.
//
// http://github.com/marcopaganini/smartcollector
// (C) 2016 by Marco Paganini <paganini@paganini.net>

package main

import (
	"flag"
	"fmt"
	"github.com/marcopaganini/gosmart"
	"golang.org/x/net/context"
	"log"
	"os"
	"path/filepath"
)

const (
	// Prefix for the token file under the home directory.
	tokenFilePrefix = ".smartcollector"

	// Where your node exporter looks for textfile collector files. This is
	// passed to node_exporter with the -collector.textfile.directory
	// command-line argument.
	textFileCollectorDir = "/run/textfile_collector"

	// Time series textfile collector filename
	textFileCollectorName = "smartcollector.prom"
)

var (
	flagClient               = flag.String("client", "", "OAuth Client ID")
	flagSecret               = flag.String("secret", "", "OAuth Secret")
	flagTextFileCollectorDir = flag.String("textfile-dir", textFileCollectorDir, "Textfile Collector directory")
	flagDryRun               = flag.Bool("dry-run", false, "Just print the values (don't save to file)")
)

func main() {
	flag.Parse()

	// No date on log messages
	log.SetFlags(0)

	if *flagClient == "" {
		log.Fatalf("Must specify Client ID (--client)")
	}
	tfile := tokenFilePrefix + "_" + *flagClient + ".json"

	// Create the oauth2.config object and get a token
	config := gosmart.NewOAuthConfig(*flagClient, *flagSecret)
	token, err := gosmart.GetToken(tfile, config)
	if err != nil {
		log.Fatalf("Error fetching token: %v", err)
	}

	// Create a client with the token and fetch endpoints URI.
	ctx := context.Background()
	client := config.Client(ctx, token)
	endpoint, err := gosmart.GetEndPointsURI(client)
	if err != nil {
		log.Fatalf("Error reading endpoints URI: %v\n", err)
	}

	// Iterate over all devices and collect timeseries info.
	devs, err := gosmart.GetDevices(client, endpoint)
	if err != nil {
		log.Fatalf("Error reading list of devices: %v\n", err)
	}

	ts := []string{}

	for _, dev := range devs {
		devinfo, err := gosmart.GetDeviceInfo(client, endpoint, dev.ID)
		if err != nil {
			log.Fatalf("Error reading device info: %v\n", err)
		}
		t, err := getTimeSeries(devinfo)
		if err != nil {
			log.Fatalf("Error processing sensor data: %v\n", err)
		}
		for _, v := range t {
			ts = append(ts, v)
		}
	}

	// Save timeseries (or just print if dry-run active)
	if *flagDryRun {
		for _, v := range ts {
			fmt.Println(v)
		}
	} else {
		f := filepath.Join(*flagTextFileCollectorDir, textFileCollectorName)
		if err := saveTimeSeries(f, ts); err != nil {
			log.Fatalf("Error saving timeseries: %v\n", err)
		}
	}
}

// saveTimeSeries saves the array of strings to a temporary file and renames
// the resulting file into a node exporter textfile collector file.
func saveTimeSeries(fname string, ts []string) error {
	// Silly temp name. Uniqueness should be sufficient (famous last words...)
	tempfile := fmt.Sprintf("%s-%d-%d", fname, os.Getpid(), os.Getppid())

	// Create file and write every ts line into it, adding newline.
	w, err := os.Create(tempfile)
	if err != nil {
		return err
	}
	defer w.Close()
	for _, v := range ts {
		w.Write([]byte(v + "\n"))
	}
	w.Close()

	// Rename to real name
	err = os.Rename(tempfile, fname)
	if err != nil {
		return err
	}
	return nil
}

// getTimeSeries returns a prometheus compatible timeseries from the device data.
func getTimeSeries(devinfo *gosmart.DeviceInfo) ([]string, error) {
	var err error
	var value float64

	valOpenClosed := []string{"open", "closed"}
	valInactiveActive := []string{"inactive", "active"}
	valAbsentPresent := []string{"absent", "present"}
	valOffOn := []string{"off", "on"}

	ret := []string{}

	for k, val := range devinfo.Attributes {
		// Some sensors report nil as a value (instead of a blank string) so we
		// convert nil to an empty string to avoid issues with type assertion.
		if val == nil {
			val = ""
		}

		switch k {
		case "alarmState":
			value, err = valueClear(val)
		case "battery":
			value, err = valueFloat(val)
		case "carbonMonoxide":
			value, err = valueClear(val)
		case "contact":
			value, err = valueOneOf(val, valOpenClosed)
		case "energy":
			value, err = valueFloat(val)
		case "motion":
			value, err = valueOneOf(val, valInactiveActive)
		case "power":
			value, err = valueFloat(val)
		case "presence":
			value, err = valueOneOf(val, valAbsentPresent)
		case "smoke":
			value, err = valueClear(val)
		case "switch":
			value, err = valueOneOf(val, valOffOn)
		case "temperature":
			value, err = valueFloat(val)
		default:
			// We only process keys we know about.
			continue
		}
		if err != nil {
			return nil, err
		}
		ret = append(ret, fmt.Sprintf("smartthings_sensors{id=\"%s\" name=\"%s\" attr=\"%v\"} = %v", devinfo.ID, devinfo.DisplayName, k, value))
	}
	return ret, nil
}

// valueClear expects a string and returns 0 for "clear", 1 for anything else.
// TODO: Expand this to properly identify non-clear conditions and return error
// in case an unexpected value is found.
func valueClear(v interface{}) (float64, error) {
	val, ok := v.(string)
	if !ok {
		return 0.0, fmt.Errorf("invalid non-string argument %v", v)
	}
	if val != "clear" {
		return 0.0, nil
	}
	return 1.0, nil
}

// valueOneOf returns 0.0 if the value matches the first item
// in the array, 1.0 if it matches the second, and an error if
// nothing matches.
func valueOneOf(v interface{}, options []string) (float64, error) {
	val, ok := v.(string)
	if !ok {
		return 0.0, fmt.Errorf("invalid non-string argument %v", v)
	}
	if val == options[0] {
		return 0.0, nil
	}
	if val == options[1] {
		return 1.0, nil
	}
	return 0.0, fmt.Errorf("invalid option %q. Expected %q or %q", val, options[0], options[1])
}

// valueFloat returns the float64 value of the value passed or
// error if the value cannot be converted.
func valueFloat(v interface{}) (float64, error) {
	val, ok := v.(float64)
	if !ok {
		return 0.0, fmt.Errorf("invalid non floating-point argument %v", v)
	}
	return val, nil
}
