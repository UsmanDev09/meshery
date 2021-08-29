// Copyright 2020 Layer5, Inc.
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

package system

import (
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/layer5io/meshery/mesheryctl/internal/cli/root/config"
	"github.com/layer5io/meshery/mesheryctl/pkg/utils"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configuration     *config.MesheryCtlConfig
	tempCntxt         = "local"
	set               = false
	adapters          = []string{}
	platform          = ""
	serverUrl         = ""
	newContext        = ""
	currContext       string
	allContext        bool
	tokenNameLocation = map[string]string{} //maps each token name to its specified location
)

type contextWithLocation struct {
	Endpoint      string   `mapstructure:"endpoint,omitempty"`
	Token         string   `mapstructure:"token,omitempty"`
	Tokenlocation string   `mapstructure:"token,omitempty" yaml:"token-location,omitempty"`
	Platform      string   `mapstructure:"platform"`
	Adapters      []string `mapstructure:"adapters,omitempty"`
	Channel       string   `mapstructure:"channel,omitempty"`
	Version       string   `mapstructure:"version,omitempty"`
}

// createContextCmd represents the create command
var createContextCmd = &cobra.Command{
	Use:   "create <context-name>",
	Short: "Create a new context (a named Meshery deployment)",
	Long:  `Add a new context to Meshery config.yaml file`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tempCntxt := utils.TemplateContext

		if serverUrl != "" {
			err := utils.ValidateURL(serverUrl)
			if err != nil {
				return err
			}
			tempCntxt.Endpoint = serverUrl
		}

		log.Debug("serverUrl: `" + tempCntxt.Endpoint + "`")

		if platform != "" {
			tempCntxt.Platform = platform
		}

		if len(adapters) >= 1 {
			tempCntxt.Adapters = adapters
		}

		err := config.AddContextToConfig(args[0], tempCntxt, viper.ConfigFileUsed(), set)
		if err != nil {
			return err
		}

		log.Printf("Added `%s` context", args[0])
		return nil
	},
}

// deleteContextCmd represents the delete command
var deleteContextCmd = &cobra.Command{
	Use:   "delete <context-name>",
	Short: "delete context",
	Long:  `Delete an existing context (a named Meshery deployment) from Meshery config file`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(&configuration)
		if err != nil {
			return err
		}
		_, exists := configuration.Contexts[args[0]]
		if !exists {
			return errors.New("no context to delete")
		}

		if viper.GetString("current-context") == args[0] {
			var res bool
			if utils.SilentFlag {
				res = true
			} else {
				res = utils.AskForConfirmation("Are you sure you want to delete the current context")
			}

			if !res {
				log.Printf("Delete aborted")
				return nil
			}

			var result string

			if newContext != "" {
				_, exists := configuration.Contexts[newContext]
				if !exists {
					return errors.New("new context wrongly set")
				}

				if newContext == args[0] {
					return errors.New("choose a new context other than the context being deleted")
				}

				result = newContext
			} else {
				var listContexts []string
				for context := range configuration.Contexts {
					if context != args[0] {
						listContexts = append(listContexts, context)
					}
				}

				prompt := promptui.Select{
					Label: "Select context",
					Items: listContexts,
				}

				_, result, err = prompt.Run()

				if err != nil {
					fmt.Printf("Prompt failed %v\n", err)
					return err
				}
			}

			fmt.Printf("The current context is now %q\n", result)
			viper.Set("current-context", result)
		}
		delete(configuration.Contexts, args[0])
		viper.Set("contexts", configuration.Contexts)
		log.Printf("deleted context %s", args[0])
		err = viper.WriteConfig()

		return err
	},
}

// listContextCmd represents the list command
var listContextCmd = &cobra.Command{
	Use:          "list",
	Short:        "list contexts",
	Long:         `List current context and available contexts`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(&configuration)
		if err != nil {
			return err
		}
		var contexts = configuration.Contexts
		if contexts == nil {
			// return errors.New("no available contexts")
			log.Print("No contexts available. Use `mesheryctl system context create <name>` to create a new Meshery deployment context.\n")
			return nil
		}

		if currContext == "" {
			currContext = viper.GetString("current-context")
		}
		if currContext == "" {
			log.Print("Current context not set\n")
		} else {
			log.Printf("Current context: %s\n", currContext)
		}
		log.Print("Available contexts:\n")

		//sorting the contexts to get a consistent order on each subsequent run
		keys := make([]string, 0, len(contexts))
		for k := range contexts {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			log.Printf("- %s", k)
		}

		if currContext == "" {
			log.Print("\nRun `mesheryctl system context switch <context name>` to set the current context.")
		}
		return nil
	},
}

// viewContextCmd represents the view command
var viewContextCmd = &cobra.Command{
	Use:          "view",
	Short:        "view current context",
	Long:         `Display active Meshery context`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(&configuration)
		if err != nil {
			return err
		}
		//Storing all the tokens separately in a map, to get tokenlocation by token name.
		for _, tok := range configuration.Tokens {
			tokenNameLocation[tok.Name] = tok.Location
		}

		if allContext {
			tempcontexts := make(map[string]contextWithLocation)

			//Populating auxiliary struct with token-locations
			for k, v := range configuration.Contexts {
				if v.Token == "" {
					log.Warnf("[Warning]: Token not specified/empty for context \"%s\"", k)
					temp, _ := getContextWithTokenLocation(&v)
					tempcontexts[k] = *temp
				} else {
					temp, ok := getContextWithTokenLocation(&v)
					tempcontexts[k] = *temp
					if !ok {
						log.Warnf("[Warning]: Token \"%s\" could not be found! for context \"%s\"", tempcontexts[k].Token, k)
					}
				}

			}

			log.Print(getYAML(tempcontexts))

			return nil
		}
		if len(args) != 0 {
			currContext = args[0]
		}
		if currContext == "" {
			currContext = viper.GetString("current-context")

		}
		if currContext == "" {
			return errors.New("current context not set")
		}

		contextData, ok := configuration.Contexts[currContext]
		if !ok {
			log.Printf("context \"%s\" doesn't exists, run the following to create:\n\nmesheryctl system context create %s", currContext, currContext)
			return nil
		}

		if contextData.Token == "" {
			log.Warnf("[Warning]: Token not specified/empty for context \"%s\"", currContext)
			log.Printf("\nCurrent Context: %s\n", currContext)
			log.Print(getYAML(contextData))
		} else {
			temp, ok := getContextWithTokenLocation(&contextData)
			log.Printf("\nCurrent Context: %s\n", currContext)
			if !ok {
				log.Warnf("[Warning]: Token \"%s\" could not be found! for context \"%s\"", temp.Token, currContext)
			}
			log.Print(getYAML(temp))
		}

		return nil
	},
}

// switchContextCmd represents the switch command
var switchContextCmd = &cobra.Command{
	Use:          "switch <context-name>",
	Short:        "switch context",
	Long:         `Configure mesheryctl to actively use one one context vs. the another context`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(&configuration)
		if err != nil {
			return err
		}
		_, exists := configuration.Contexts[args[0]]
		if !exists {
			return errors.New("requested context does not exist")
		}
		if viper.GetString("current-context") == args[0] {
			return errors.New("already using context '" + args[0] + "'")
		}
		if err := stop(); err != nil {
			return errors.Wrap(err, utils.SystemError("Failed to stop Meshery before switching context"))
		}
		configuration.CurrentContext = args[0]
		viper.Set("current-context", configuration.CurrentContext)
		log.Printf("switched to context '%s'", args[0])
		err = viper.WriteConfig()
		if Starterr := start(); Starterr != nil {
			return errors.Wrap(Starterr, utils.SystemError("Failed to start Meshery while switching context"))
		}
		return err
	},
}

// ContextCmd represents the context command
var ContextCmd = &cobra.Command{
	Use:   "context [command]",
	Short: "Configure your Meshery deployment(s)",
	Long:  `Configure and switch between different named Meshery server and adapter versions and deployments.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			currentContext := viper.GetString("current-context")
			if currentContext == "" {
				return errors.New("current context not set")
			}

			log.Printf("Current context: %s\n", currentContext)
			return cmd.Help()
		}

		if ok := utils.IsValidSubcommand(availableSubcommands, args[0]); !ok {
			return errors.New(utils.SystemError(fmt.Sprintf("invalid command: \"%s\"", args[0])))
		}
		return nil
	},
}

func init() {
	availableSubcommands = []*cobra.Command{
		createContextCmd,
		deleteContextCmd,
		switchContextCmd,
		viewContextCmd,
		listContextCmd,
	}
	createContextCmd.Flags().StringVarP(&serverUrl, "url", "u", "", "Meshery Server URL with Port")
	createContextCmd.Flags().BoolVarP(&set, "set", "s", false, "Set as current context")
	createContextCmd.Flags().StringArrayVarP(&adapters, "adapters", "a", []string{}, "List of adapters")
	createContextCmd.Flags().StringVarP(&platform, "platform", "p", "", "Platform to deploy Meshery")
	deleteContextCmd.Flags().StringVarP(&newContext, "set", "s", "", "New context to deploy Meshery")
	viewContextCmd.Flags().StringVarP(&currContext, "context", "c", "", "Show config for the context")
	viewContextCmd.Flags().BoolVar(&allContext, "all", false, "Show configs for all of the context")
	ContextCmd.PersistentFlags().StringVarP(&tempCntxt, "context", "c", "", "(optional) temporarily change the current context.")
	ContextCmd.AddCommand(availableSubcommands...)
}

// getYAML takes in a struct and converts it into yaml
func getYAML(strct interface{}) string {
	out, _ := yaml.Marshal(strct)
	return string(out)
}

func getContextWithTokenLocation(c *config.Context) (*contextWithLocation, bool) {
	temp := contextWithLocation{
		Endpoint:      c.Endpoint,
		Token:         c.Token,
		Tokenlocation: tokenNameLocation[c.Token],
		Platform:      c.Platform,
		Adapters:      c.Adapters,
		Channel:       c.Channel,
		Version:       c.Version,
	}
	if temp.Tokenlocation == "" {
		return &temp, false
	}
	return &temp, true
}
