// Copyright © 2017 SBB Cloud Stack Team
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"errors"

	"github.com/op/go-logging"
	"github.com/oscp/openshift-monitoring-checks/checks"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pretty	bool
var debug	bool

var log = logging.MustGetLogger("openshift-monitoring-cli")

// the data type for single shot events
type EventData map[string]interface{}

// defines the format of the output JSON integrations will return
type IntegrationData struct {
	Name               string      `json:"name"`
	ProtocolVersion    string      `json:"protocol_version"`
	IntegrationVersion string      `json:"integration_version"`
	Events             []EventData `json:"events"`
}

var data = IntegrationData{
	Name:               "ch.sbb.openshift-integration",
	ProtocolVersion:    "1",
	IntegrationVersion: "1.0.0",
	Events:             make([]EventData, 0),
}

var rootCmd = &cobra.Command{
	Use:   "openshift-monitoring-cli",
	Short: "This cli tool runs monitoring checks for OpenShift installations.",
	Long: `This application will run major and minor checks for different node types like
for example worker node, master or storage node.`,
	Run: runChecks,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Critical(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig, initLogging)

	rootCmd.Flags().BoolVarP(&pretty, "pretty", "p", false, "print pretty json output")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "print debug messages")
}

func initLogging() {
	var format = logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} - %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	stdOutBackend := logging.NewLogBackend(os.Stdout, "", 0)
	logging.SetBackend(logging.NewBackendFormatter(stdOutBackend, format))

	if runtime.GOOS != "windows" {
		sysLogBackend, err := logging.NewSyslogBackend("openshift-monitoring-cli")

		if err != nil {
			log.Warning("Wasn't able to initialize syslog.", err)
		} else {
			if debug {
				sysLogFormatter := logging.NewBackendFormatter(sysLogBackend, format)
				stdOutFormatter := logging.NewBackendFormatter(stdOutBackend, format)
				logging.SetBackend(logging.MultiLogger(sysLogFormatter, stdOutFormatter))
			} else {
				logging.SetBackend(logging.NewBackendFormatter(sysLogBackend, format))
			}
		}
	}

	if viper.GetString("logging.level") == "debug" {
		logging.SetLevel(logging.DEBUG, "openshift-monitoring-cli")
	} else {
		logging.SetLevel(logging.INFO, "openshift-monitoring-cli")
	}

}

func initConfig() {
	ex, err := os.Executable()

	if err != nil {
		log.Critical(err)
		os.Exit(1)
	}

	viper.AddConfigPath(filepath.Dir(ex))
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		log.Error("Not able to read config file (path of script is", filepath.Dir(ex)+")", "config.yml.")
		log.Critical(err)
		os.Exit(1)
	}

}

func createEvent(err error) map[string]interface{} {
	var event = map[string]interface{}{}
	event["summary"] = err.Error()
	return event
}

func createHealthyEvent(err error) map[string]interface{} {
	var event = createEvent(err)
	event["category"] = "HEALTHY"
	log.Error("HEALTHY:", err.Error())
	return event
}

func evalMajor(fn func() error) {
	if err := fn(); err != nil {
		var event= createEvent(err)
		event["category"] = "MAJOR"
		log.Error("MAJOR:", err.Error())
		data.Events = append(data.Events, event)
	}
}

func evalMinor(fn func() error) {
	if err := fn(); err != nil {
		var event = createEvent(err)
		event["category"] = "MINOR"
		log.Error("MINOR:", err.Error())
		data.Events = append(data.Events, event)
	}
}

func runChecks(cmd *cobra.Command, args []string) {
	log.Info("Running", viper.GetString("node.type"), "checks for OpenShift.")

	if viper.GetString("node.type") == "master" {
		if len(viper.GetString("etcd.ips")) == 0 || len(viper.GetString("router.ips")) == 0 {
			log.Fatal("Can't read service IPs from configuration file.")
		}
	}

	/////////////////
	//// MAJORS ////
	////////////////
	log.Debug("Running major checks.")

	// majors on storage
	if viper.GetString("node.type") == "storage" {
		log.Debug("Running major checks for storage.")

		evalMajor(func() error { return checks.CheckIfGlusterdIsRunning() })
		evalMajor(func() error { return checks.CheckMountPointSizes(90) })
		evalMajor(func() error { return checks.CheckLVPoolSizes(90) })
		evalMajor(func() error { return checks.CheckVGSizes(5) })
	}

	// majors on node
	if viper.GetString("node.type") == "node" {
		log.Debug("Running major checks for node.")

		evalMajor(func() error { return checks.CheckDockerPool(90) })
		evalMajor(func() error { return checks.CheckDnsNslookupOnKubernetes() })
		evalMajor(func() error { return checks.CheckDnsServiceNode() })
		evalMajor(func() error { return checks.CheckSslCertificates(viper.GetStringSlice("certs.paths.node"), viper.GetInt("certs.majorDays")) })
	}

	// majors on master
	if viper.GetString("node.type") == "master" {
		log.Debug("Running major checks for master.")

		evalMajor(func() error { return checks.CheckOcGetNodes() })
		evalMajor(func() error { return checks.CheckEtcdHealth(viper.GetString("etcd.ips"), "") })

		if len(viper.GetString("registry.ip")) > 0 {
			evalMajor(func() error { return checks.CheckRegistryHealth(viper.GetString("registry.ip")) })
		}

		for _, rip := range strings.Split(viper.GetString("router.ips"), ",") {
			evalMajor(func() error { return checks.CheckRouterHealth(rip) })
		}

		evalMajor(func() error { return checks.CheckMasterApis("https://localhost:8443/api") })
		evalMajor(func() error { return checks.CheckDnsNslookupOnKubernetes() })
		evalMajor(func() error { return checks.CheckDnsServiceNode() })
		evalMajor(func() error { return checks.CheckSslCertificates(viper.GetStringSlice("certs.paths.master"), viper.GetInt("certs.majorDays")) })
	}

	/////////////////
	//// MINORS ////
	////////////////
	log.Debug("Running minor checks.")

	// minors on storage
	if viper.GetString("node.type") == "storage" {
		log.Debug("Running minor checks for storage.")

		evalMinor(func() error { return checks.CheckOpenFileCount() })
		evalMinor(func() error { return checks.CheckMountPointSizes(85) })
		evalMinor(func() error { return checks.CheckLVPoolSizes(80) })
		evalMinor(func() error { return checks.CheckVGSizes(10) })
	}

	// minors on node
	if viper.GetString("node.type") == "node" {
		log.Debug("Running minor checks for node.")

		evalMinor(func() error { return checks.CheckDockerPool(80) })
		evalMinor(func() error { return checks.CheckHttpService(false) })
		evalMajor(func() error { return checks.CheckSslCertificates(viper.GetStringSlice("certs.paths.node"), viper.GetInt("certs.minorDays")) })
	}

	// minors on master
	if viper.GetString("node.type") == "master" {
		log.Debug("Running minor checks for master.")

		evalMinor(func() error { return checks.CheckExternalSystem(viper.GetString("externalSystemUrl")) })
		evalMinor(func() error { return checks.CheckHawcularHealth(viper.GetString("hawcularIP")) })
		evalMinor(func() error { return checks.CheckRouterRestartCount() })
		evalMinor(func() error { return checks.CheckLimitsAndQuotas(viper.GetInt("projectsWithoutLimits")) })
		evalMinor(func() error { return checks.CheckHttpService(false) })
		evalMinor(func() error { return checks.CheckLoggingRestartsCount() })
		evalMajor(func() error { return checks.CheckSslCertificates(viper.GetStringSlice("certs.paths.master"), viper.GetInt("certs.minorDays")) })
	}

	log.Debug("Running minor checks for all node types.")
	// minor for all server types
	evalMinor(func() error { return checks.CheckNtpd() })

	if len(data.Events) == 0 {
		data.Events = append(data.Events, createHealthyEvent(errors.New("System healthy, nothing to do.")));
	}

	OutputJSON(data)
}

func OutputJSON(data interface{}) {
	var output []byte
	var err error

	if pretty {
		output, err = json.MarshalIndent(data, "", "\t")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		log.Errorf("Error outputting JSON (%s).", err)
	}

	if string(output) == "null" {
		fmt.Print("[]")
	} else {
		fmt.Print(string(output))
	}
}
