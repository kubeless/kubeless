/*
Copyright (c) 2016-2017 Bitnami

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cronjob

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cronjobApi "github.com/kubeless/cronjob-trigger/pkg/apis/kubeless/v1beta1"
	cronjobUtils "github.com/kubeless/cronjob-trigger/pkg/utils"
	kubelessUtils "github.com/kubeless/kubeless/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var createCmd = &cobra.Command{
	Use:   "create <cronjob_trigger_name> FLAG",
	Short: "Create a cron job trigger",
	Long:  `Create a cron job trigger`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 1 {
			logrus.Fatal("Need exactly one argument - cronjob trigger name")
		}
		triggerName := args[0]

		schedule, err := cmd.Flags().GetString("schedule")
		if err != nil {
			logrus.Fatal(err)
		}

		if _, err := cron.ParseStandard(schedule); err != nil {
			logrus.Fatalf("Invalid value for --schedule. " + err.Error())
		}

		ns, err := cmd.Flags().GetString("namespace")
		if err != nil {
			logrus.Fatal(err)
		}
		if ns == "" {
			ns = kubelessUtils.GetDefaultNamespace()
		}

		functionName, err := cmd.Flags().GetString("function")
		if err != nil {
			logrus.Fatal(err)
		}

		dryrun, err := cmd.Flags().GetBool("dryrun")
		if err != nil {
			logrus.Fatal(err)
		}

		output, err := cmd.Flags().GetString("output")
		if err != nil {
			logrus.Fatal(err)
		}

		payload, err := cmd.Flags().GetString("payload")
		if err != nil {
			logrus.Fatal(err)
		}

		payloadFromFile, err := cmd.Flags().GetString("payload-from-file")
		if err != nil {
			logrus.Fatal(err)
		}

		kubelessClient, err := kubelessUtils.GetKubelessClientOutCluster()
		if err != nil {
			logrus.Fatalf("Can not create out-of-cluster client: %v", err)
		}

		cronJobClient, err := cronjobUtils.GetKubelessClientOutCluster()
		if err != nil {
			logrus.Fatalf("Can not create out-of-cluster client: %v", err)
		}

		_, err = kubelessUtils.GetFunctionCustomResource(kubelessClient, functionName, ns)
		if err != nil {
			logrus.Fatalf("Unable to find Function %s in namespace %s. Error %s", functionName, ns, err)
		}

		parsedPayload := parsePayload(payload, payloadFromFile)

		cronJobTrigger := cronjobApi.CronJobTrigger{}
		cronJobTrigger.TypeMeta = metav1.TypeMeta{
			Kind:       "CronJobTrigger",
			APIVersion: "kubeless.io/v1beta1",
		}
		cronJobTrigger.ObjectMeta = metav1.ObjectMeta{
			Name:      triggerName,
			Namespace: ns,
		}
		cronJobTrigger.ObjectMeta.Labels = map[string]string{
			"created-by": "kubeless",
		}
		cronJobTrigger.Spec.FunctionName = functionName
		cronJobTrigger.Spec.Schedule = schedule
		cronJobTrigger.Spec.Payload = parsedPayload

		if dryrun == true {
			res, err := kubelessUtils.DryRunFmt(output, cronJobTrigger)
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(res)
			return
		}

		err = cronjobUtils.CreateCronJobCustomResource(cronJobClient, &cronJobTrigger)
		if err != nil {
			logrus.Fatalf("Failed to create cronjob trigger object %s in namespace %s. Error: %s", triggerName, ns, err)
		}
		logrus.Infof("Cronjob trigger %s created in namespace %s successfully!", triggerName, ns)
	},
}

func init() {
	createCmd.Flags().StringP("namespace", "n", "", "Specify namespace for the cronjob trigger")
	createCmd.Flags().StringP("schedule", "", "", "Specify schedule in cron format for scheduled function")
	createCmd.Flags().StringP("function", "", "", "Name of the function to be associated with trigger")
	createCmd.MarkFlagRequired("function")
	createCmd.MarkFlagRequired("schedule")
	createCmd.Flags().Bool("dryrun", false, "Output JSON manifest of the function without creating it")
	createCmd.Flags().StringP("output", "o", "yaml", "Output format")
	createCmd.Flags().StringP("payload", "p", "", "Specify a stringified JSON data to pass to function upon execution")
	createCmd.Flags().StringP("payload-from-file", "f", "", "Specify a payload file to use. It must be a JSON file")
}

func parsePayload(raw string, file string) interface{} {
	content, err := getPayloadRawContent(raw, file)
	if err != nil {
		return fmt.Errorf("Found an error while parsing your payload: %s", err)
	}

	return parsePayloadContent(content)
}

func getPayloadRawContent(content string, file string) (string, error) {
	if len(content) == 0 {
		origin := getOrigin(file)
		content, err := getPayloadFileContent(file, origin)
		if err != nil {
			return "", err
		}

		ext := filepath.Ext(file)
		if ext != ".json" {
			return "", fmt.Errorf("Sorry, we can't parse %s files yet", ext)
		}

		return content, nil
	}

	return content, nil
}

func getOrigin(file string) string {
	var origin string

	isURL := strings.HasPrefix(file, "https://") || strings.HasPrefix(file, "http://")

	if isURL {
		origin = "url"
	} else {
		origin = "file"
	}

	return origin
}

func getPayloadFileContent(file string, origin string) (string, error) {
	var content string

	if origin == "url" {
		payloadURL, err := url.Parse(file)
		if err != nil {
			return "", err
		}
		resp, err := http.Get(payloadURL.String())
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		payloadBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		content = string(payloadBytes)
	} else {
		payloadBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return "", err
		}

		content = string(payloadBytes)
	}

	return content, nil
}

func parsePayloadContent(raw string) interface{} {
	var payload map[string]interface{}

	err := json.Unmarshal([]byte(raw), &payload)
	if err != nil {
		return fmt.Errorf("Found an error during JSON parsing on your payload: %s", err)
	}

	return payload
}
